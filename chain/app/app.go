package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/bettergov/chain/x/bank"
	"github.com/bettergov/chain/x/evm"
	"github.com/bettergov/chain/x/gov"
	"github.com/bettergov/chain/x/mint"
	"github.com/bettergov/chain/x/slashing"
	"github.com/bettergov/chain/x/staking"
	"github.com/bettergov/chain/x/transparency"
)

type GenesisAccount struct {
	Address     string `json:"address"`
	BalanceAbgov string `json:"balance_abgov"`
}

type GenesisValidator struct {
	OperatorAddress   string                  `json:"operator_address"`
	ConsensusPubKey   string                  `json:"consensus_pubkey_hex"`
	Moniker           string                  `json:"moniker"`
	Role              string                  `json:"role"`
	Website           string                  `json:"website"`
	SecurityContact   string                  `json:"security_contact"`
	Details           string                  `json:"details"`
	SelfBondAbgov     string                  `json:"self_bond_abgov"`
	MinSelfDelegation string                  `json:"min_self_delegation_abgov"`
	Commission        staking.CommissionRates `json:"commission"`
}

type GenesisState struct {
	ChainID          string                  `json:"chain_id"`
	EVMChainID       uint64                  `json:"evm_chain_id"`
	GenesisTime      time.Time               `json:"genesis_time"`
	EconomicParams   mint.EconomicParams     `json:"economic_params"`
	StakingParams    staking.StakingParams   `json:"staking_params"`
	SlashingParams   slashing.SlashingParams `json:"slashing_params"`
	GovParams        gov.GovParams           `json:"gov_params"`
	Accounts         []GenesisAccount        `json:"accounts"`
	Validators       []GenesisValidator      `json:"validators"`
	InitialAuditors  []string                `json:"initial_auditors"`
}

type BlockHeader struct {
	ChainID         string    `json:"chain_id"`
	Height          int64     `json:"height"`
	Time            time.Time `json:"time"`
	ProposerAddress string    `json:"proposer_address"`
	LastBlockHash   string    `json:"last_block_hash"`
	AppHash         string    `json:"app_hash"`
	BlockHash       string    `json:"block_hash"`
	TxCount         int       `json:"tx_count"`
	GasUsed         uint64    `json:"gas_used"`
}

type SnapshotBundle struct {
	ChainID      string            `json:"chain_id"`
	EVMChainID   uint64            `json:"evm_chain_id"`
	Height       int64             `json:"height"`
	AppHash      string            `json:"app_hash"`
	CreatedAt    time.Time         `json:"created_at"`
	Balances     map[string]string `json:"balances"`
}

type BetterGovApp struct {
	mu           sync.RWMutex
	ChainID      string
	EVMChainID   uint64
	Height       int64
	CurrentTime  time.Time
	LastAppHash  string
	Blocks       []BlockHeader

	Bank         *bank.Keeper
	Mint         *mint.Keeper
	Staking      *staking.Keeper
	Slashing     *slashing.Keeper
	Gov          *gov.Keeper
	Transparency *transparency.Keeper
	EVM          *evm.Keeper
}

func NewBetterGovApp(genesis GenesisState) (*BetterGovApp, error) {
	if err := genesis.EconomicParams.Validate(); err != nil {
		return nil, fmt.Errorf("invalid economic_params: %w", err)
	}

	bKeeper := bank.NewKeeper()
	mKeeper := mint.NewKeeper(bKeeper, genesis.EconomicParams)
	stKeeper := staking.NewKeeper(bKeeper, genesis.StakingParams)
	slKeeper := slashing.NewKeeper(stKeeper, genesis.SlashingParams)
	govKeeper := gov.NewKeeper(bKeeper, stKeeper, genesis.GovParams)
	trKeeper := transparency.NewKeeper()
	evmKeeper := evm.NewKeeper(bKeeper, genesis.EVMChainID, big.NewInt(1_000_000_000))

	app := &BetterGovApp{
		ChainID:      genesis.ChainID,
		EVMChainID:   genesis.EVMChainID,
		Height:       0,
		CurrentTime:  genesis.GenesisTime,
		Blocks:       []BlockHeader{},
		Bank:         bKeeper,
		Mint:         mKeeper,
		Staking:      stKeeper,
		Slashing:     slKeeper,
		Gov:          govKeeper,
		Transparency: trKeeper,
		EVM:          evmKeeper,
	}

	govKeeper.SetParamChangeHook(app.applyGovernanceParamChanges)

	// Mint genesis accounts
	for _, acc := range genesis.Accounts {
		amt, err := bank.ParseAbgov(acc.BalanceAbgov)
		if err != nil {
			return nil, fmt.Errorf("invalid genesis account balance for %s: %w", acc.Address, err)
		}
		if amt.Sign() > 0 {
			if err := bKeeper.MintTokens(acc.Address, amt); err != nil {
				return nil, err
			}
		}
	}

	// Register genesis validators
	for _, gv := range genesis.Validators {
		selfBond, err := bank.ParseAbgov(gv.SelfBondAbgov)
		if err != nil {
			return nil, err
		}
		minSelf, err := bank.ParseAbgov(gv.MinSelfDelegation)
		if err != nil {
			return nil, err
		}
		val, err := stKeeper.CreateValidatorWithRole(
			gv.OperatorAddress,
			gv.ConsensusPubKey,
			gv.Moniker,
			gv.Role,
			gv.Website,
			gv.SecurityContact,
			gv.Details,
			selfBond,
			minSelf,
			gv.Commission,
			genesis.GenesisTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to register genesis validator %s: %w", gv.Moniker, err)
		}
		slKeeper.EnsureSigningInfo(val.ConsensusAddress, 1)
	}

	for _, aud := range genesis.InitialAuditors {
		trKeeper.RegisterAuditor(aud)
	}

	app.LastAppHash = app.ComputeAppHash()
	return app, nil
}

func (a *BetterGovApp) applyGovernanceParamChanges(changes []gov.ParameterChange) error {
	for _, c := range changes {
		switch c.Subspace {
		case "mint":
			p := a.Mint.GetParams()
			val, err := strconv.ParseUint(c.Value, 10, 64)
			if err != nil {
				return err
			}
			switch c.Key {
			case "inflation_rate_bps":
				p.InflationRateBps = val
			case "community_tax_bps":
				p.CommunityTaxBps = val
			case "goal_bonded_bps":
				p.GoalBondedBps = val
			default:
				return fmt.Errorf("unknown mint parameter key: %s", c.Key)
			}
			if err := a.Mint.SetParams(p); err != nil {
				return err
			}
		case "staking":
			p := a.Staking.GetParams()
			val, err := strconv.ParseUint(c.Value, 10, 64)
			if err != nil {
				return err
			}
			switch c.Key {
			case "max_validators":
				p.MaxValidators = uint32(val)
			case "min_commission_rate_bps":
				p.MinCommissionRateBps = val
			case "unbonding_time_seconds":
				p.UnbondingTimeSeconds = int64(val)
			default:
				return fmt.Errorf("unknown staking parameter key: %s", c.Key)
			}
			a.Staking.SetParams(p)
		default:
			return fmt.Errorf("unknown parameter subspace: %s", c.Subspace)
		}
	}
	return nil
}

// ComputeAppHash computes a deterministic Merkle-like root over all 7 protocol module state roots.
func (a *BetterGovApp) ComputeAppHash() string {
	roots := struct {
		Bank         string `json:"bank"`
		Staking      string `json:"staking"`
		Slashing     string `json:"slashing"`
		Gov          string `json:"gov"`
		Transparency string `json:"transparency"`
		EVM          string `json:"evm"`
	}{
		Bank:         a.Bank.StateRoot(),
		Staking:      a.Staking.StateRoot(),
		Slashing:     a.Slashing.StateRoot(),
		Gov:          a.Gov.StateRoot(),
		Transparency: a.Transparency.StateRoot(),
		EVM:          a.EVM.StateRoot(),
	}
	raw, _ := json.Marshal(roots)
	sum := sha256.Sum256(raw)
	return "0x" + hex.EncodeToString(sum[:])
}

// FinalizeBlock executes a full CometBFT ABCI block lifecycle (BeginBlock -> DeliverTxs -> EndBlock -> Commit).
func (a *BetterGovApp) FinalizeBlock(blockTime time.Time, evmTxs []*evm.EVMTransaction, absentValidatorConsAddrs map[string]bool) (*BlockHeader, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	nextHeight := a.Height + 1
	if blockTime.IsZero() {
		blockTime = a.CurrentTime.Add(5 * time.Second)
	}
	a.CurrentTime = blockTime

	// 1. BeginBlock: Mint block provisions and check validator liveness signatures
	bondedTokens := a.Staking.GetTotalBondedTokens()
	_, err := a.Mint.MintBlockProvision(bondedTokens)
	if err != nil {
		return nil, err
	}

	activeUpdates := a.Staking.GetValidatorUpdates()
	proposerAddr := "bgovvalcons1genesis"
	if len(activeUpdates) > 0 {
		proposerIdx := int(nextHeight) % len(activeUpdates)
		proposerAddr = activeUpdates[proposerIdx].ConsensusAddress
	}

	for _, valUp := range activeUpdates {
		signed := true
		if absentValidatorConsAddrs != nil && absentValidatorConsAddrs[valUp.ConsensusAddress] {
			signed = false
		}
		_, _ = a.Slashing.HandleValidatorSignature(valUp.ConsensusAddress, signed, nextHeight, blockTime)
	}

	// 2. DeliverTx: Execute EVM transactions
	lastBlockHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	if len(a.Blocks) > 0 {
		lastBlockHash = a.Blocks[len(a.Blocks)-1].BlockHash
	}
	prelimBlockHash := fmt.Sprintf("0x%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", a.ChainID, nextHeight, blockTime.UnixNano()))))

	totalGasUsed := uint64(0)
	for _, tx := range evmTxs {
		rc, err := a.EVM.ApplyTransaction(tx, uint64(nextHeight), prelimBlockHash)
		if err != nil {
			return nil, err
		}
		totalGasUsed += rc.GasUsed
	}

	// 3. EndBlock: Distribute block rewards, mature unbonding/redelegation queues, and tally governance proposals
	_, _ = a.Staking.DistributeBlockRewards()
	a.Staking.CompleteMatureQueues(blockTime)
	a.Gov.EndBlocker(blockTime)

	// 4. Commit: Compute new AppHash and append block header
	appHash := a.ComputeAppHash()
	a.LastAppHash = appHash
	a.Height = nextHeight

	header := BlockHeader{
		ChainID:         a.ChainID,
		Height:          nextHeight,
		Time:            blockTime,
		ProposerAddress: proposerAddr,
		LastBlockHash:   lastBlockHash,
		AppHash:         appHash,
		BlockHash:       prelimBlockHash,
		TxCount:         len(evmTxs),
		GasUsed:         totalGasUsed,
	}
	a.Blocks = append(a.Blocks, header)
	return &header, nil
}

func (a *BetterGovApp) ExportSnapshot() SnapshotBundle {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return SnapshotBundle{
		ChainID:    a.ChainID,
		EVMChainID: a.EVMChainID,
		Height:     a.Height,
		AppHash:    a.LastAppHash,
		CreatedAt:  a.CurrentTime,
		Balances:   a.Bank.ExportBalances(),
	}
}

func (a *BetterGovApp) RestoreSnapshot(snap SnapshotBundle) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.Bank.ImportBalances(snap.Balances); err != nil {
		return err
	}
	a.Height = snap.Height
	a.LastAppHash = a.ComputeAppHash()
	return nil
}

// DefaultDevnetGenesis returns a ready-to-run local development genesis state with 4 validators and pre-funded test accounts.
func DefaultDevnetGenesis() GenesisState {
	genTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stParams := staking.DefaultParams()
	stParams.UnbondingTimeSeconds = 60 // 60s for local devnet testing

	govParams := gov.DefaultParams()
	govParams.VotingPeriodSec = 120 // 2m voting period on devnet

	return GenesisState{
		ChainID:        "bettergov-devnet-1",
		EVMChainID:     31337,
		GenesisTime:    genTime,
		EconomicParams: mint.DefaultEconomicParams(),
		StakingParams:  stParams,
		SlashingParams: slashing.DefaultParams(),
		GovParams:      govParams,
		Accounts: []GenesisAccount{
			{Address: "0x70997970c51812dc3a010c7d01b50e0d17dc79c8", BalanceAbgov: bank.BGOVToAbgov(10_000_000).String()},
			{Address: "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc", BalanceAbgov: bank.BGOVToAbgov(10_000_000).String()},
			{Address: "0x90f79bf6eb2c4f870365e785982e1f101e93b906", BalanceAbgov: bank.BGOVToAbgov(10_000_000).String()},
			{Address: "0x15d34aaf54267db7d7c367839aaf71a00a2c6a65", BalanceAbgov: bank.BGOVToAbgov(10_000_000).String()},
			{Address: "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266", BalanceAbgov: bank.BGOVToAbgov(25_000_000).String()},
			{Address: bank.PoolTreasury, BalanceAbgov: bank.BGOVToAbgov(20_000_000).String()},
		},
		Validators: []GenesisValidator{
			{
				OperatorAddress:   "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
				ConsensusPubKey:   "1111111111111111111111111111111111111111111111111111111111111111",
				Moniker:           "Sentinel-Alpha-Node",
				Role:              staking.RoleConsensusGuardian,
				Website:           "https://alpha.validator.bettergov.org",
				SecurityContact:   "sec-ops-alpha",
				Details:           "Primary Civic Infrastructure Validator — Manila Zone",
				SelfBondAbgov:     bank.BGOVToAbgov(2_500_000).String(),
				MinSelfDelegation: bank.BGOVToAbgov(50_000).String(),
				Commission:        staking.CommissionRates{RateBps: 500, MaxRateBps: 2000, MaxChangeRateBps: 100},
			},
			{
				OperatorAddress:   "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc",
				ConsensusPubKey:   "2222222222222222222222222222222222222222222222222222222222222222",
				Moniker:           "CivicTrust-Validator",
				Role:              staking.RoleProcurementWatchdog,
				Website:           "https://civictrust.bettergov.org",
				SecurityContact:   "sec-ops-civictrust",
				Details:           "Public Accountability & Open Data Validator — Cebu Zone",
				SelfBondAbgov:     bank.BGOVToAbgov(2_000_000).String(),
				MinSelfDelegation: bank.BGOVToAbgov(50_000).String(),
				Commission:        staking.CommissionRates{RateBps: 700, MaxRateBps: 2000, MaxChangeRateBps: 100},
			},
			{
				OperatorAddress:   "0x90f79bf6eb2c4f870365e785982e1f101e93b906",
				ConsensusPubKey:   "3333333333333333333333333333333333333333333333333333333333333333",
				Moniker:           "Aegis-GovTech-Staking",
				Role:              staking.RoleTreasurySentinel,
				Website:           "https://aegis.bettergov.org",
				SecurityContact:   "sec-ops-aegis",
				Details:           "High-Availability Institutional Validator — Davao Zone",
				SelfBondAbgov:     bank.BGOVToAbgov(1_800_000).String(),
				MinSelfDelegation: bank.BGOVToAbgov(50_000).String(),
				Commission:        staking.CommissionRates{RateBps: 600, MaxRateBps: 1500, MaxChangeRateBps: 100},
			},
			{
				OperatorAddress:   "0x15d34aaf54267db7d7c367839aaf71a00a2c6a65",
				ConsensusPubKey:   "4444444444444444444444444444444444444444444444444444444444444444",
				Moniker:           "OpenLedger-Guard",
				Role:              staking.RolePublicAuditorNode,
				Website:           "https://openledger.bettergov.org",
				SecurityContact:   "sec-ops-openledger",
				Details:           "Independent Academic & Audit Validator",
				SelfBondAbgov:     bank.BGOVToAbgov(1_500_000).String(),
				MinSelfDelegation: bank.BGOVToAbgov(50_000).String(),
				Commission:        staking.CommissionRates{RateBps: 500, MaxRateBps: 2000, MaxChangeRateBps: 100},
			},
		},
		InitialAuditors: []string{
			"0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
			"0x15d34aaf54267db7d7c367839aaf71a00a2c6a65",
		},
	}
}

