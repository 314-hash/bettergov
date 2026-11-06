package bank

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
)

const (
	TokenName     = "BetterGov"
	TokenSymbol   = "BGOV"
	BaseDenom     = "abgov"
	TokenDecimals = 18

	// Protocol Module Pool Addresses
	PoolBondedTokens    = "bgov1pool_bonded_tokens_0000000000000000"
	PoolNotBondedTokens = "bgov1pool_not_bonded_tokens_000000000000"
	PoolGovDeposit      = "bgov1pool_gov_deposit_000000000000000000"
	PoolTreasury        = "bgov1pool_community_treasury_00000000000"
	PoolFeeCollector    = "bgov1pool_fee_collector_0000000000000000"
)

var (
	ErrInsufficientFunds           = errors.New("insufficient BGOV (abgov) balance")
	ErrInvalidAmount               = errors.New("amount must be strictly positive")
	ErrInvalidAddress              = errors.New("invalid account address")
	ErrBSP1108FiatRailsProhibited  = errors.New("BSP Circular No. 1108 guard: direct fiat (PHP) rails and remittance settlement are prohibited at the protocol layer; fiat conversions must occur off-chain via licensed VASPs")
)

// ValidateBSP1108CryptoUtilityOnly enforces that protocol operations strictly remain at the crypto-utility layer
// with zero direct PHP fiat rails or unlicensed remittance settlement.
func ValidateBSP1108CryptoUtilityOnly(denomOrTag string) error {
	upper := strings.ToUpper(strings.TrimSpace(denomOrTag))
	if upper == "PHP" || strings.Contains(upper, "FIAT_PHP") || strings.Contains(upper, "PHP_REMITTANCE") {
		return ErrBSP1108FiatRailsProhibited
	}
	return nil
}

// OneBGOV returns 10^18 abgov (1 BGOV).
func OneBGOV() *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(TokenDecimals), nil)
}

// BGOVToAbgov converts whole BGOV units to 18-decimal abgov atomic units.
func BGOVToAbgov(bgov int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(bgov), OneBGOV())
}

// ParseAbgov parses a decimal string in abgov into a *big.Int.
func ParseAbgov(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 {
		return nil, fmt.Errorf("invalid abgov amount: %s", s)
	}
	return v, nil
}

// Keeper manages account balances and total supply for the native BGOV asset.
type Keeper struct {
	mu          sync.RWMutex
	balances    map[string]*big.Int
	totalSupply *big.Int
	totalBurned *big.Int
}

func NewKeeper() *Keeper {
	return &Keeper{
		balances:    make(map[string]*big.Int),
		totalSupply: big.NewInt(0),
		totalBurned: big.NewInt(0),
	}
}

func NormalizeAddress(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

func (k *Keeper) GetBalance(addr string) *big.Int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	addr = NormalizeAddress(addr)
	bal, ok := k.balances[addr]
	if !ok {
		return big.NewInt(0)
	}
	return new(big.Int).Set(bal)
}

func (k *Keeper) GetTotalSupply() *big.Int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return new(big.Int).Set(k.totalSupply)
}

func (k *Keeper) GetTotalBurned() *big.Int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return new(big.Int).Set(k.totalBurned)
}

// MintTokens mints new abgov into the specified recipient address and increases TotalSupply.
func (k *Keeper) MintTokens(recipient string, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return ErrInvalidAmount
	}
	recipient = NormalizeAddress(recipient)
	if recipient == "" {
		return ErrInvalidAddress
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	cur, ok := k.balances[recipient]
	if !ok {
		cur = big.NewInt(0)
	}
	k.balances[recipient] = new(big.Int).Add(cur, amount)
	k.totalSupply.Add(k.totalSupply, amount)
	return nil
}

// BurnTokens burns abgov from a target address (used in slashing and vetoed deposits).
func (k *Keeper) BurnTokens(holder string, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return ErrInvalidAmount
	}
	holder = NormalizeAddress(holder)

	k.mu.Lock()
	defer k.mu.Unlock()

	cur, ok := k.balances[holder]
	if !ok || cur.Cmp(amount) < 0 {
		return ErrInsufficientFunds
	}
	k.balances[holder] = new(big.Int).Sub(cur, amount)
	k.totalSupply.Sub(k.totalSupply, amount)
	k.totalBurned.Add(k.totalBurned, amount)
	return nil
}

// Transfer moves abgov between two accounts atomically.
func (k *Keeper) Transfer(from, to string, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return ErrInvalidAmount
	}
	from = NormalizeAddress(from)
	to = NormalizeAddress(to)
	if from == "" || to == "" {
		return ErrInvalidAddress
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	fromBal, ok := k.balances[from]
	if !ok || fromBal.Cmp(amount) < 0 {
		return ErrInsufficientFunds
	}
	k.balances[from] = new(big.Int).Sub(fromBal, amount)

	toBal, ok := k.balances[to]
	if !ok {
		toBal = big.NewInt(0)
	}
	k.balances[to] = new(big.Int).Add(toBal, amount)
	return nil
}

// ExportBalances returns a deterministic sorted map of all non-zero balances.
func (k *Keeper) ExportBalances() map[string]string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	out := make(map[string]string, len(k.balances))
	for addr, bal := range k.balances {
		if bal.Sign() > 0 {
			out[addr] = bal.String()
		}
	}
	return out
}

// ImportBalances restores balances and recalculates totalSupply.
func (k *Keeper) ImportBalances(balances map[string]string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.balances = make(map[string]*big.Int)
	k.totalSupply = big.NewInt(0)
	for addr, raw := range balances {
		val, err := ParseAbgov(raw)
		if err != nil {
			return err
		}
		norm := NormalizeAddress(addr)
		k.balances[norm] = val
		k.totalSupply.Add(k.totalSupply, val)
	}
	return nil
}

// StateRoot computes a deterministic SHA-256 commitment over all balances and total supply.
func (k *Keeper) StateRoot() string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	addrs := make([]string, 0, len(k.balances))
	for a, b := range k.balances {
		if b.Sign() > 0 {
			addrs = append(addrs, a)
		}
	}
	sort.Strings(addrs)

	type entry struct {
		Address string `json:"address"`
		Balance string `json:"balance"`
	}
	entries := make([]entry, 0, len(addrs))
	for _, a := range addrs {
		entries = append(entries, entry{Address: a, Balance: k.balances[a].String()})
	}
	payload, _ := json.Marshal(struct {
		TotalSupply string  `json:"total_supply"`
		TotalBurned string  `json:"total_burned"`
		Balances    []entry `json:"balances"`
	}{
		TotalSupply: k.totalSupply.String(),
		TotalBurned: k.totalBurned.String(),
		Balances:    entries,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

