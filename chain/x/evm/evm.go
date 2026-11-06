package evm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"

	"github.com/bettergov/chain/x/bank"
)

const (
	BaseTxGas         uint64 = 21000
	ContractCreateGas uint64 = 53000
	CalldataByteGas   uint64 = 16
	SStoreGas         uint64 = 20000
)

var (
	ErrChainIDMismatch = errors.New("EIP-155 chain ID mismatch")
	ErrNonceMismatch   = errors.New("invalid transaction nonce (replay or out-of-order)")
	ErrIntrinsicGasLow = errors.New("gas limit below intrinsic gas requirement")
	ErrOutOfGas        = errors.New("out of gas during EVM execution")
	ErrExecutionRevert = errors.New("EVM execution reverted")
)

type EVMLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber uint64   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
	LogIndex    uint     `json:"logIndex"`
}

type EVMTransaction struct {
	Hash      string   `json:"hash"`
	ChainID   uint64   `json:"chainId"`
	Nonce     uint64   `json:"nonce"`
	From      string   `json:"from"`
	To        string   `json:"to"` // Empty for contract deployment
	Value     *big.Int `json:"value"`
	DataHex   string   `json:"data"`
	GasLimit  uint64   `json:"gas"`
	GasPrice  *big.Int `json:"gasPrice"`
	Signature string   `json:"signature"`
}

func (tx *EVMTransaction) ComputeHash() string {
	raw, _ := json.Marshal(struct {
		ChainID  uint64 `json:"c"`
		Nonce    uint64 `json:"n"`
		From     string `json:"f"`
		To       string `json:"t"`
		Value    string `json:"v"`
		DataHex  string `json:"d"`
		GasLimit uint64 `json:"g"`
		GasPrice string `json:"p"`
	}{
		ChainID:  tx.ChainID,
		Nonce:    tx.Nonce,
		From:     strings.ToLower(tx.From),
		To:       strings.ToLower(tx.To),
		Value:    tx.Value.String(),
		DataHex:  tx.DataHex,
		GasLimit: tx.GasLimit,
		GasPrice: tx.GasPrice.String(),
	})
	sum := sha256.Sum256(raw)
	return "0x" + hex.EncodeToString(sum[:])
}

type TransactionReceipt struct {
	TransactionHash   string   `json:"transactionHash"`
	BlockNumber       uint64   `json:"blockNumber"`
	BlockHash         string   `json:"blockHash"`
	From              string   `json:"from"`
	To                string   `json:"to,omitempty"`
	ContractAddress   string   `json:"contractAddress,omitempty"`
	CumulativeGasUsed uint64   `json:"cumulativeGasUsed"`
	GasUsed           uint64   `json:"gasUsed"`
	EffectiveGasPrice string   `json:"effectiveGasPrice"`
	Status            uint64   `json:"status"` // 1 = success, 0 = revert
	ReturnDataHex     string   `json:"returnData,omitempty"`
	Logs              []EVMLog `json:"logs"`
}

type Keeper struct {
	mu           sync.RWMutex
	chainID      uint64
	minGasPrice  *big.Int
	bank         *bank.Keeper
	nonces       map[string]uint64
	code         map[string]string            // contractAddr -> hex bytecode
	storage      map[string]map[string]string // contractAddr -> slotHex -> valueHex
	transactions map[string]*EVMTransaction
	receipts     map[string]*TransactionReceipt
}

func NewKeeper(b *bank.Keeper, chainID uint64, minGasPrice *big.Int) *Keeper {
	if minGasPrice == nil {
		minGasPrice = big.NewInt(1_000_000_000) // 1 gwei default
	}
	return &Keeper{
		chainID:      chainID,
		minGasPrice:  minGasPrice,
		bank:         b,
		nonces:       make(map[string]uint64),
		code:         make(map[string]string),
		storage:      make(map[string]map[string]string),
		transactions: make(map[string]*EVMTransaction),
		receipts:     make(map[string]*TransactionReceipt),
	}
}

func (k *Keeper) ChainID() uint64 {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.chainID
}

func (k *Keeper) MinGasPrice() *big.Int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return new(big.Int).Set(k.minGasPrice)
}

func (k *Keeper) GetNonce(addr string) uint64 {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.nonces[bank.NormalizeAddress(addr)]
}

func (k *Keeper) GetCode(addr string) string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	c, ok := k.code[bank.NormalizeAddress(addr)]
	if !ok {
		return "0x"
	}
	return c
}

func (k *Keeper) GetStorageAt(addr, slot string) string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	addr = bank.NormalizeAddress(addr)
	slot = strings.ToLower(strings.TrimPrefix(slot, "0x"))
	if st, ok := k.storage[addr]; ok {
		if val, exists := st[slot]; exists {
			return val
		}
	}
	return "0x" + strings.Repeat("0", 64)
}

func DeriveContractAddress(sender string, nonce uint64) string {
	preimage := fmt.Sprintf("CREATE:%s:%d", bank.NormalizeAddress(sender), nonce)
	sum := sha256.Sum256([]byte(preimage))
	return "0x" + hex.EncodeToString(sum[12:])
}

func IntrinsicGas(dataHex string, isCreate bool) uint64 {
	clean := strings.TrimPrefix(dataHex, "0x")
	byteLen := uint64(len(clean) / 2)
	base := BaseTxGas
	if isCreate {
		base = ContractCreateGas
	}
	return base + (byteLen * CalldataByteGas)
}

// ApplyTransaction executes an Ethereum-compatible transaction against state and produces a deterministic receipt.
func (k *Keeper) ApplyTransaction(tx *EVMTransaction, blockNumber uint64, blockHash string) (*TransactionReceipt, error) {
	if tx.Value == nil {
		tx.Value = big.NewInt(0)
	}
	if tx.GasPrice == nil || tx.GasPrice.Cmp(k.minGasPrice) < 0 {
		tx.GasPrice = new(big.Int).Set(k.minGasPrice)
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if tx.ChainID != k.chainID {
		return nil, ErrChainIDMismatch
	}
	sender := bank.NormalizeAddress(tx.From)
	expectedNonce := k.nonces[sender]
	if tx.Nonce != expectedNonce {
		return nil, fmt.Errorf("%w: got %d, expected %d", ErrNonceMismatch, tx.Nonce, expectedNonce)
	}

	isCreate := strings.TrimSpace(tx.To) == ""
	intrinsic := IntrinsicGas(tx.DataHex, isCreate)
	if tx.GasLimit < intrinsic {
		return nil, ErrIntrinsicGasLow
	}

	// Calculate max gas fee upfront and verify balance >= Value + GasUsed*GasPrice
	gasUsed := intrinsic
	cleanData := strings.TrimPrefix(tx.DataHex, "0x")
	if len(cleanData) > 0 && !isCreate {
		gasUsed += SStoreGas
	}
	if gasUsed > tx.GasLimit {
		return nil, ErrOutOfGas
	}

	gasFee := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), tx.GasPrice)
	totalRequired := new(big.Int).Add(tx.Value, gasFee)
	if k.bank.GetBalance(sender).Cmp(totalRequired) < 0 {
		return nil, bank.ErrInsufficientFunds
	}

	// Pay gas fee into PoolFeeCollector (for validators & delegators)
	if gasFee.Sign() > 0 {
		if err := k.bank.Transfer(sender, bank.PoolFeeCollector, gasFee); err != nil {
			return nil, err
		}
	}

	// Increment sender nonce
	k.nonces[sender] = expectedNonce + 1

	txHash := tx.ComputeHash()
	tx.Hash = txHash

	receipt := &TransactionReceipt{
		TransactionHash:   txHash,
		BlockNumber:       blockNumber,
		BlockHash:         blockHash,
		From:              sender,
		CumulativeGasUsed: gasUsed,
		GasUsed:           gasUsed,
		EffectiveGasPrice: "0x" + tx.GasPrice.Text(16),
		Status:            1,
		Logs:              []EVMLog{},
	}

	if isCreate {
		contractAddr := DeriveContractAddress(sender, expectedNonce)
		receipt.ContractAddress = contractAddr
		if tx.Value.Sign() > 0 {
			if err := k.bank.Transfer(sender, contractAddr, tx.Value); err != nil {
				receipt.Status = 0
			}
		}
		if receipt.Status == 1 {
			k.code[contractAddr] = "0x" + cleanData
			k.storage[contractAddr] = make(map[string]string)
			receipt.ReturnDataHex = "0x" + cleanData
			receipt.Logs = append(receipt.Logs, EVMLog{
				Address:     contractAddr,
				Topics:      []string{"0x" + sha256HexString("ContractDeployed(address,address)")},
				Data:        "0x" + hex.EncodeToString([]byte(sender)),
				BlockNumber: blockNumber,
				TxHash:      txHash,
				LogIndex:    0,
			})
		}
	} else {
		toAddr := bank.NormalizeAddress(tx.To)
		receipt.To = toAddr
		if tx.Value.Sign() > 0 {
			if err := k.bank.Transfer(sender, toAddr, tx.Value); err != nil {
				receipt.Status = 0
			}
		}
		// If calling a deployed contract with calldata, update deterministic contract storage & emit log
		if receipt.Status == 1 && len(cleanData) >= 8 {
			selector := cleanData[:8]
			argPayload := cleanData[8:]
			if _, ok := k.storage[toAddr]; !ok {
				k.storage[toAddr] = make(map[string]string)
			}
			slotHash := sha256HexString(selector)
			valHash := sha256HexString(argPayload)
			k.storage[toAddr][slotHash] = "0x" + valHash
			receipt.ReturnDataHex = "0x" + valHash
			receipt.Logs = append(receipt.Logs, EVMLog{
				Address:     toAddr,
				Topics:      []string{"0x" + slotHash},
				Data:        "0x" + valHash,
				BlockNumber: blockNumber,
				TxHash:      txHash,
				LogIndex:    0,
			})
		}
	}

	txCopy := *tx
	k.transactions[txHash] = &txCopy
	k.receipts[txHash] = receipt
	return receipt, nil
}

// EthCall executes a read-only call without mutating state.
func (k *Keeper) EthCall(to string, dataHex string) (string, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	toAddr := bank.NormalizeAddress(to)
	cleanData := strings.TrimPrefix(dataHex, "0x")
	if len(cleanData) >= 8 {
		selector := cleanData[:8]
		slotHash := sha256HexString(selector)
		if st, ok := k.storage[toAddr]; ok {
			if val, exists := st[slotHash]; exists {
				return val, nil
			}
		}
	}
	if code, ok := k.code[toAddr]; ok && code != "0x" {
		sum := sha256HexString(code + cleanData)
		return "0x" + sum, nil
	}
	return "0x" + strings.Repeat("0", 64), nil
}

func (k *Keeper) GetTransaction(hash string) (*EVMTransaction, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	tx, ok := k.transactions[strings.ToLower(hash)]
	return tx, ok
}

func (k *Keeper) GetReceipt(hash string) (*TransactionReceipt, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	rc, ok := k.receipts[strings.ToLower(hash)]
	return rc, ok
}

func (k *Keeper) GetAllReceipts() []*TransactionReceipt {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]*TransactionReceipt, 0, len(k.receipts))
	for _, r := range k.receipts {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BlockNumber != out[j].BlockNumber {
			return out[i].BlockNumber > out[j].BlockNumber
		}
		return out[i].TransactionHash < out[j].TransactionHash
	})
	return out
}

func sha256HexString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (k *Keeper) StateRoot() string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	addrs := make([]string, 0, len(k.code))
	for a := range k.code {
		addrs = append(addrs, a)
	}
	sort.Strings(addrs)

	payload, _ := json.Marshal(struct {
		ChainID   uint64            `json:"chain_id"`
		Contracts []string          `json:"contracts"`
		Nonces    map[string]uint64 `json:"nonces"`
		TxCount   int               `json:"tx_count"`
	}{
		ChainID:   k.chainID,
		Contracts: addrs,
		Nonces:    k.nonces,
		TxCount:   len(k.transactions),
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

