package staking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/bettergov/chain/x/bank"
)

type BondStatus string

const (
	BondStatusBonded    BondStatus = "BOND_STATUS_BONDED"
	BondStatusUnbonding BondStatus = "BOND_STATUS_UNBONDING"
	BondStatusUnbonded  BondStatus = "BOND_STATUS_UNBONDED"

	// Official BETTERGOV Validator Roles
	RoleConsensusGuardian   = "CONSENSUS_GUARDIAN"
	RolePublicAuditorNode   = "PUBLIC_AUDITOR_NODE"
	RoleProcurementWatchdog = "PROCUREMENT_WATCHDOG"
	RoleTreasurySentinel    = "TREASURY_SENTINEL"
	RoleRegionalCivicRelay  = "REGIONAL_CIVIC_RELAY"
	RolePostQuantumNotary   = "POST_QUANTUM_NOTARY"
)

type ValidatorRoleSpec struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func AvailableValidatorRoles() []ValidatorRoleSpec {
	return []ValidatorRoleSpec{
		{
			Code:        RoleConsensusGuardian,
			Title:       "Consensus & Finality Guardian",
			Description: "Secures BFT block production, EVM state execution, and composite AppHash finality.",
		},
		{
			Code:        RolePublicAuditorNode,
			Title:       "Public Fiscal & Audit Verifier",
			Description: "Independently verifies public documents and upgrades records from UNVERIFIED_HASH_ANCHOR to AUDITOR_VERIFIED.",
		},
		{
			Code:        RoleProcurementWatchdog,
			Title:       "Procurement & Infrastructure Watchdog",
			Description: "Monitors public tender bids, contract awards, and regional project milestone disbursements.",
		},
		{
			Code:        RoleTreasurySentinel,
			Title:       "Community Treasury & BSP 1108 Sentinel",
			Description: "Validates milestone-gated treasury disbursements and enforces BSP Circular No. 1108 crypto-utility separation.",
		},
		{
			Code:        RoleRegionalCivicRelay,
			Title:       "Regional LGU & Citizen Transparency Relay",
			Description: "Relays pseudonymous citizen participation records while enforcing the Cleartext PII Guard.",
		},
		{
			Code:        RolePostQuantumNotary,
			Title:       "Post-Quantum Identity & SBT Notary",
			Description: "Attests NIST post-quantum key slots (Dilithium, Falcon, SPHINCS+) and ERC-5192 Soulbound civic credentials.",
		},
	}
}

func NormalizeValidatorRole(role string) string {
	switch role {
	case RoleConsensusGuardian,
		RolePublicAuditorNode,
		RoleProcurementWatchdog,
		RoleTreasurySentinel,
		RoleRegionalCivicRelay,
		RolePostQuantumNotary:
		return role
	default:
		if role != "" {
			return role
		}
		return RoleConsensusGuardian
	}
}

var (
	ErrValidatorExists          = errors.New("validator already registered")
	ErrConsensusKeyExists       = errors.New("consensus public key already in use by another validator")
	ErrValidatorNotFound        = errors.New("validator not found")
	ErrValidatorJailed          = errors.New("validator is jailed")
	ErrValidatorTombstoned      = errors.New("validator is permanently tombstoned for double-signing")
	ErrBelowMinSelfDelegation   = errors.New("self-delegation is below validator minimum self-delegation")
	ErrInvalidCommission        = errors.New("invalid validator commission parameters")
	ErrCommissionChangeTooLarge = errors.New("commission rate change exceeds max_change_rate_bps")
	ErrInsufficientShares       = errors.New("insufficient delegation shares")
	ErrTransitiveRedelegation   = errors.New("cannot redelegate from a validator that is currently a destination of an active redelegation")
	ErrSelfRedelegation         = errors.New("cannot redelegate to the same validator")
)

type StakingParams struct {
	MaxValidators         uint32 `json:"max_validators"`           // 0 = UNLIMITED active validators; >0 = optional cap
	UnlimitedValidators   bool   `json:"unlimited_validators"`     // True when active validator set has no artificial cap
	UnbondingTimeSeconds  int64  `json:"unbonding_time_seconds"`   // Unbonding lock duration in seconds
	MaxEntries            uint32 `json:"max_entries"`              // Max concurrent unbonding/redelegation entries per pair
	MinCommissionRateBps  uint64 `json:"min_commission_rate_bps"`  // Protocol minimum commission (e.g. 500 = 5.00%)
	BondDenom             string `json:"bond_denom"`               // Must be "abgov"
}

func DefaultParams() StakingParams {
	return StakingParams{
		MaxValidators:        0,       // 0 = UNLIMITED validators allowed in active consensus
		UnlimitedValidators:  true,    // Open permissionless civic validator set
		UnbondingTimeSeconds: 1814400, // 21 days default
		MaxEntries:           7,
		MinCommissionRateBps: 500,     // 5% minimum validator commission
		BondDenom:            bank.BaseDenom,
	}
}

type CommissionRates struct {
	RateBps          uint64    `json:"rate_bps"`
	MaxRateBps       uint64    `json:"max_rate_bps"`
	MaxChangeRateBps uint64    `json:"max_change_rate_bps"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Validator struct {
	OperatorAddress         string          `json:"operator_address"`
	ConsensusAddress        string          `json:"consensus_address"`
	ConsensusPubKeyHex      string          `json:"consensus_pubkey_hex"`
	Moniker                 string          `json:"moniker"`
	Role                    string          `json:"role"`
	Website                 string          `json:"website"`
	SecurityContact         string          `json:"security_contact"`
	Details                 string          `json:"details"`
	Jailed                  bool            `json:"jailed"`
	Tombstoned              bool            `json:"tombstoned"`
	Status                  BondStatus      `json:"status"`
	Tokens                  *big.Int        `json:"tokens"`
	DelegatorShares         *big.Int        `json:"delegator_shares"`
	MinSelfDelegation       *big.Int        `json:"min_self_delegation"`
	Commission              CommissionRates `json:"commission"`
	AccumulatedCommission   *big.Int        `json:"accumulated_commission"`
	AccumulatedRewards      *big.Int        `json:"accumulated_rewards"`
	SlashCount              uint64          `json:"slash_count"`
}

// VotingPower returns consensus voting power in whole BGOV (Tokens / 10^18).
func (v *Validator) VotingPower() int64 {
	if v.Jailed || v.Tombstoned || v.Status != BondStatusBonded {
		return 0
	}
	vp := new(big.Int).Div(v.Tokens, bank.OneBGOV())
	return vp.Int64()
}

// TokensFromShares calculates the token value of a given share amount.
func (v *Validator) TokensFromShares(shares *big.Int) *big.Int {
	if v.DelegatorShares.Sign() == 0 {
		return big.NewInt(0)
	}
	num := new(big.Int).Mul(shares, v.Tokens)
	return new(big.Int).Div(num, v.DelegatorShares)
}

type Delegation struct {
	DelegatorAddress      string   `json:"delegator_address"`
	ValidatorAddress      string   `json:"validator_address"`
	Shares                *big.Int `json:"shares"`
	PendingRewards        *big.Int `json:"pending_rewards"`
	ActivityBoostBps      uint64   `json:"activity_boost_bps"`       // SEC Mitigation: Activity-based reward booster (0 - 2500 bps)
	VerifiedContributions uint64   `json:"verified_contributions"`   // Count of verified active contributions (clean energy, crops, voting)
	LastActivityProofHash string   `json:"last_activity_proof_hash"` // SHA-256 receipt of active contribution
}

type UnbondingEntry struct {
	CreationHeight int64     `json:"creation_height"`
	CompletionTime time.Time `json:"completion_time"`
	InitialBalance *big.Int  `json:"initial_balance"`
	Balance        *big.Int  `json:"balance"`
}

type UnbondingDelegation struct {
	DelegatorAddress string           `json:"delegator_address"`
	ValidatorAddress string           `json:"validator_address"`
	Entries          []UnbondingEntry `json:"entries"`
}

type RedelegationEntry struct {
	CreationHeight int64     `json:"creation_height"`
	CompletionTime time.Time `json:"completion_time"`
	InitialBalance *big.Int  `json:"initial_balance"`
	SharesDst      *big.Int  `json:"shares_dst"`
}

type Redelegation struct {
	DelegatorAddress    string              `json:"delegator_address"`
	ValidatorSrcAddress string              `json:"validator_src_address"`
	ValidatorDstAddress string              `json:"validator_dst_address"`
	Entries             []RedelegationEntry `json:"entries"`
}

type ValidatorUpdate struct {
	OperatorAddress    string `json:"operator_address"`
	ConsensusAddress   string `json:"consensus_address"`
	ConsensusPubKeyHex string `json:"consensus_pubkey_hex"`
	Role               string `json:"role"`
	VotingPower        int64  `json:"voting_power"`
}

type Keeper struct {
	mu            sync.RWMutex
	params        StakingParams
	bank          *bank.Keeper
	validators    map[string]*Validator
	consAddrIndex map[string]string // consensusAddr -> operatorAddr
	delegations   map[string]*Delegation
	unbondings    map[string]*UnbondingDelegation
	redelegations map[string]*Redelegation
}

func NewKeeper(b *bank.Keeper, params StakingParams) *Keeper {
	return &Keeper{
		params:        params,
		bank:          b,
		validators:    make(map[string]*Validator),
		consAddrIndex: make(map[string]string),
		delegations:   make(map[string]*Delegation),
		unbondings:    make(map[string]*UnbondingDelegation),
		redelegations: make(map[string]*Redelegation),
	}
}

func DeriveConsensusAddress(pubKeyHex string) (string, error) {
	raw, err := hex.DecodeString(pubKeyHex)
	if err != nil || len(raw) != 32 {
		return "", fmt.Errorf("consensus public key must be 32-byte hex (Ed25519)")
	}
	sum := sha256.Sum256(raw)
	return "bgovvalcons1" + hex.EncodeToString(sum[:20]), nil
}

func delKey(delAddr, valAddr string) string {
	return bank.NormalizeAddress(delAddr) + "|" + bank.NormalizeAddress(valAddr)
}

func redelKey(delAddr, srcVal, dstVal string) string {
	return bank.NormalizeAddress(delAddr) + "|" + bank.NormalizeAddress(srcVal) + "|" + bank.NormalizeAddress(dstVal)
}

func (k *Keeper) GetParams() StakingParams {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.params
}

func (k *Keeper) SetParams(p StakingParams) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.params = p
}

// CreateValidator registers a new validator with self-bonded BGOV (defaults role to CONSENSUS_GUARDIAN).
func (k *Keeper) CreateValidator(
	operatorAddr string,
	pubKeyHex string,
	moniker string,
	website string,
	securityContact string,
	details string,
	selfBond *big.Int,
	minSelfDelegation *big.Int,
	commission CommissionRates,
	blockTime time.Time,
) (*Validator, error) {
	return k.CreateValidatorWithRole(
		operatorAddr,
		pubKeyHex,
		moniker,
		RoleConsensusGuardian,
		website,
		securityContact,
		details,
		selfBond,
		minSelfDelegation,
		commission,
		blockTime,
	)
}

// CreateValidatorWithRole registers a new validator with an explicit BETTERGOV institutional role.
func (k *Keeper) CreateValidatorWithRole(
	operatorAddr string,
	pubKeyHex string,
	moniker string,
	role string,
	website string,
	securityContact string,
	details string,
	selfBond *big.Int,
	minSelfDelegation *big.Int,
	commission CommissionRates,
	blockTime time.Time,
) (*Validator, error) {
	operatorAddr = bank.NormalizeAddress(operatorAddr)
	consAddr, err := DeriveConsensusAddress(pubKeyHex)
	if err != nil {
		return nil, err
	}
	if minSelfDelegation == nil || minSelfDelegation.Sign() <= 0 {
		return nil, ErrBelowMinSelfDelegation
	}
	if selfBond == nil || selfBond.Cmp(minSelfDelegation) < 0 {
		return nil, ErrBelowMinSelfDelegation
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if _, exists := k.validators[operatorAddr]; exists {
		return nil, ErrValidatorExists
	}
	if _, exists := k.consAddrIndex[consAddr]; exists {
		return nil, ErrConsensusKeyExists
	}

	if commission.RateBps < k.params.MinCommissionRateBps ||
		commission.MaxRateBps > 10000 ||
		commission.RateBps > commission.MaxRateBps ||
		commission.MaxChangeRateBps > commission.MaxRateBps {
		return nil, ErrInvalidCommission
	}
	commission.UpdatedAt = blockTime

	// Move self-bonded tokens into the protocol BondedTokensPool
	if err := k.bank.Transfer(operatorAddr, bank.PoolBondedTokens, selfBond); err != nil {
		return nil, err
	}

	val := &Validator{
		OperatorAddress:       operatorAddr,
		ConsensusAddress:      consAddr,
		ConsensusPubKeyHex:    pubKeyHex,
		Moniker:               moniker,
		Role:                  NormalizeValidatorRole(role),
		Website:               website,
		SecurityContact:       securityContact,
		Details:               details,
		Jailed:                false,
		Tombstoned:            false,
		Status:                BondStatusBonded,
		Tokens:                new(big.Int).Set(selfBond),
		DelegatorShares:       new(big.Int).Set(selfBond),
		MinSelfDelegation:     new(big.Int).Set(minSelfDelegation),
		Commission:            commission,
		AccumulatedCommission: big.NewInt(0),
		AccumulatedRewards:    big.NewInt(0),
	}

	k.validators[operatorAddr] = val
	k.consAddrIndex[consAddr] = operatorAddr

	k.delegations[delKey(operatorAddr, operatorAddr)] = &Delegation{
		DelegatorAddress: operatorAddr,
		ValidatorAddress: operatorAddr,
		Shares:           new(big.Int).Set(selfBond),
		PendingRewards:   big.NewInt(0),
	}

	k.recomputeActiveValidatorSetLocked()
	return cloneValidator(val), nil
}

// EditCommission updates a validator's commission rate within protocol bounds.
func (k *Keeper) EditCommission(operatorAddr string, newRateBps uint64, blockTime time.Time) error {
	operatorAddr = bank.NormalizeAddress(operatorAddr)

	k.mu.Lock()
	defer k.mu.Unlock()

	val, ok := k.validators[operatorAddr]
	if !ok {
		return ErrValidatorNotFound
	}
	if newRateBps < k.params.MinCommissionRateBps || newRateBps > val.Commission.MaxRateBps {
		return ErrInvalidCommission
	}
	var diff uint64
	if newRateBps > val.Commission.RateBps {
		diff = newRateBps - val.Commission.RateBps
	} else {
		diff = val.Commission.RateBps - newRateBps
	}
	if diff > val.Commission.MaxChangeRateBps {
		return ErrCommissionChangeTooLarge
	}

	val.Commission.RateBps = newRateBps
	val.Commission.UpdatedAt = blockTime
	return nil
}

// Delegate bonds BGOV from a delegator to an active or candidate validator.
func (k *Keeper) Delegate(delegatorAddr, validatorAddr string, amount *big.Int) (*big.Int, error) {
	if amount == nil || amount.Sign() <= 0 {
		return nil, bank.ErrInvalidAmount
	}
	delegatorAddr = bank.NormalizeAddress(delegatorAddr)
	validatorAddr = bank.NormalizeAddress(validatorAddr)

	k.mu.Lock()
	defer k.mu.Unlock()

	val, ok := k.validators[validatorAddr]
	if !ok {
		return nil, ErrValidatorNotFound
	}
	if val.Tombstoned {
		return nil, ErrValidatorTombstoned
	}
	if val.Jailed {
		return nil, ErrValidatorJailed
	}

	pool := bank.PoolBondedTokens
	if val.Status != BondStatusBonded {
		pool = bank.PoolNotBondedTokens
	}
	if err := k.bank.Transfer(delegatorAddr, pool, amount); err != nil {
		return nil, err
	}

	// Calculate shares to issue: shares = (amount * val.DelegatorShares) / val.Tokens
	var newShares *big.Int
	if val.Tokens.Sign() == 0 || val.DelegatorShares.Sign() == 0 {
		newShares = new(big.Int).Set(amount)
	} else {
		num := new(big.Int).Mul(amount, val.DelegatorShares)
		newShares = new(big.Int).Div(num, val.Tokens)
	}

	val.Tokens.Add(val.Tokens, amount)
	val.DelegatorShares.Add(val.DelegatorShares, newShares)

	dk := delKey(delegatorAddr, validatorAddr)
	del, exists := k.delegations[dk]
	if !exists {
		del = &Delegation{
			DelegatorAddress: delegatorAddr,
			ValidatorAddress: validatorAddr,
			Shares:           big.NewInt(0),
			PendingRewards:   big.NewInt(0),
		}
		k.delegations[dk] = del
	}
	del.Shares.Add(del.Shares, newShares)

	k.recomputeActiveValidatorSetLocked()
	return newShares, nil
}

// Undelegate unbonds shares from a validator and places the tokens in the time-locked UnbondingQueue.
func (k *Keeper) Undelegate(delegatorAddr, validatorAddr string, sharesAmount *big.Int, height int64, blockTime time.Time) (*big.Int, time.Time, error) {
	if sharesAmount == nil || sharesAmount.Sign() <= 0 {
		return nil, time.Time{}, bank.ErrInvalidAmount
	}
	delegatorAddr = bank.NormalizeAddress(delegatorAddr)
	validatorAddr = bank.NormalizeAddress(validatorAddr)

	k.mu.Lock()
	defer k.mu.Unlock()

	val, ok := k.validators[validatorAddr]
	if !ok {
		return nil, time.Time{}, ErrValidatorNotFound
	}
	dk := delKey(delegatorAddr, validatorAddr)
	del, exists := k.delegations[dk]
	if !exists || del.Shares.Cmp(sharesAmount) < 0 {
		return nil, time.Time{}, ErrInsufficientShares
	}

	returnTokens := val.TokensFromShares(sharesAmount)
	del.Shares.Sub(del.Shares, sharesAmount)
	if del.Shares.Sign() == 0 && del.PendingRewards.Sign() == 0 {
		delete(k.delegations, dk)
	}

	val.DelegatorShares.Sub(val.DelegatorShares, sharesAmount)
	val.Tokens.Sub(val.Tokens, returnTokens)

	// Move tokens from BondedPool to NotBondedPool while unbonding
	if val.Status == BondStatusBonded && returnTokens.Sign() > 0 {
		_ = k.bank.Transfer(bank.PoolBondedTokens, bank.PoolNotBondedTokens, returnTokens)
	}

	// If validator's self-delegation drops below MinSelfDelegation, auto-jail the validator
	if delegatorAddr == validatorAddr {
		remainingSelfTokens := big.NewInt(0)
		if selfDel, ok := k.delegations[dk]; ok {
			remainingSelfTokens = val.TokensFromShares(selfDel.Shares)
		}
		if remainingSelfTokens.Cmp(val.MinSelfDelegation) < 0 && !val.Jailed {
			k.jailValidatorLocked(val)
		}
	}

	completionTime := blockTime.Add(time.Duration(k.params.UnbondingTimeSeconds) * time.Second)
	ubd, ok := k.unbondings[dk]
	if !ok {
		ubd = &UnbondingDelegation{
			DelegatorAddress: delegatorAddr,
			ValidatorAddress: validatorAddr,
		}
		k.unbondings[dk] = ubd
	}
	ubd.Entries = append(ubd.Entries, UnbondingEntry{
		CreationHeight: height,
		CompletionTime: completionTime,
		InitialBalance: new(big.Int).Set(returnTokens),
		Balance:        new(big.Int).Set(returnTokens),
	})

	k.recomputeActiveValidatorSetLocked()
	return returnTokens, completionTime, nil
}

// BeginRedelegate moves bonded stake from srcVal to dstVal with transitive redelegation protection.
func (k *Keeper) BeginRedelegate(delegatorAddr, srcValAddr, dstValAddr string, sharesAmount *big.Int, height int64, blockTime time.Time) (*big.Int, time.Time, error) {
	delegatorAddr = bank.NormalizeAddress(delegatorAddr)
	srcValAddr = bank.NormalizeAddress(srcValAddr)
	dstValAddr = bank.NormalizeAddress(dstValAddr)

	if srcValAddr == dstValAddr {
		return nil, time.Time{}, ErrSelfRedelegation
	}
	if sharesAmount == nil || sharesAmount.Sign() <= 0 {
		return nil, time.Time{}, bank.ErrInvalidAmount
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	srcVal, ok := k.validators[srcValAddr]
	if !ok {
		return nil, time.Time{}, ErrValidatorNotFound
	}
	dstVal, ok := k.validators[dstValAddr]
	if !ok {
		return nil, time.Time{}, ErrValidatorNotFound
	}
	if dstVal.Jailed || dstVal.Tombstoned {
		return nil, time.Time{}, ErrValidatorJailed
	}

	// Prevent transitive redelegation: cannot redelegate from srcVal if srcVal is currently a destination of an active redelegation
	for _, r := range k.redelegations {
		if r.DelegatorAddress == delegatorAddr && r.ValidatorDstAddress == srcValAddr && len(r.Entries) > 0 {
			for _, e := range r.Entries {
				if e.CompletionTime.After(blockTime) {
					return nil, time.Time{}, ErrTransitiveRedelegation
				}
			}
		}
	}

	srcKey := delKey(delegatorAddr, srcValAddr)
	srcDel, exists := k.delegations[srcKey]
	if !exists || srcDel.Shares.Cmp(sharesAmount) < 0 {
		return nil, time.Time{}, ErrInsufficientShares
	}

	tokensMoved := srcVal.TokensFromShares(sharesAmount)
	srcDel.Shares.Sub(srcDel.Shares, sharesAmount)
	if srcDel.Shares.Sign() == 0 && srcDel.PendingRewards.Sign() == 0 {
		delete(k.delegations, srcKey)
	}
	srcVal.DelegatorShares.Sub(srcVal.DelegatorShares, sharesAmount)
	srcVal.Tokens.Sub(srcVal.Tokens, tokensMoved)

	// Issue shares in destination validator
	var dstShares *big.Int
	if dstVal.Tokens.Sign() == 0 || dstVal.DelegatorShares.Sign() == 0 {
		dstShares = new(big.Int).Set(tokensMoved)
	} else {
		dstShares = new(big.Int).Div(new(big.Int).Mul(tokensMoved, dstVal.DelegatorShares), dstVal.Tokens)
	}
	dstVal.Tokens.Add(dstVal.Tokens, tokensMoved)
	dstVal.DelegatorShares.Add(dstVal.DelegatorShares, dstShares)

	dstKey := delKey(delegatorAddr, dstValAddr)
	dstDel, exists := k.delegations[dstKey]
	if !exists {
		dstDel = &Delegation{
			DelegatorAddress: delegatorAddr,
			ValidatorAddress: dstValAddr,
			Shares:           big.NewInt(0),
			PendingRewards:   big.NewInt(0),
		}
		k.delegations[dstKey] = dstDel
	}
	dstDel.Shares.Add(dstDel.Shares, dstShares)

	completionTime := blockTime.Add(time.Duration(k.params.UnbondingTimeSeconds) * time.Second)
	rk := redelKey(delegatorAddr, srcValAddr, dstValAddr)
	red, ok := k.redelegations[rk]
	if !ok {
		red = &Redelegation{
			DelegatorAddress:    delegatorAddr,
			ValidatorSrcAddress: srcValAddr,
			ValidatorDstAddress: dstValAddr,
		}
		k.redelegations[rk] = red
	}
	red.Entries = append(red.Entries, RedelegationEntry{
		CreationHeight: height,
		CompletionTime: completionTime,
		InitialBalance: new(big.Int).Set(tokensMoved),
		SharesDst:      new(big.Int).Set(dstShares),
	})

	k.recomputeActiveValidatorSetLocked()
	return tokensMoved, completionTime, nil
}

// CompleteMatureQueues processes matured unbondings and redelegations at EndBlock.
func (k *Keeper) CompleteMatureQueues(blockTime time.Time) int {
	k.mu.Lock()
	defer k.mu.Unlock()

	completedCount := 0
	for key, ubd := range k.unbondings {
		remaining := make([]UnbondingEntry, 0, len(ubd.Entries))
		for _, entry := range ubd.Entries {
			if !blockTime.Before(entry.CompletionTime) {
				if entry.Balance.Sign() > 0 {
					_ = k.bank.Transfer(bank.PoolNotBondedTokens, ubd.DelegatorAddress, entry.Balance)
				}
				completedCount++
			} else {
				remaining = append(remaining, entry)
			}
		}
		if len(remaining) == 0 {
			delete(k.unbondings, key)
		} else {
			ubd.Entries = remaining
		}
	}

	for key, red := range k.redelegations {
		remaining := make([]RedelegationEntry, 0, len(red.Entries))
		for _, entry := range red.Entries {
			if blockTime.Before(entry.CompletionTime) {
				remaining = append(remaining, entry)
			}
		}
		if len(remaining) == 0 {
			delete(k.redelegations, key)
		} else {
			red.Entries = remaining
		}
	}
	return completedCount
}

// DistributeBlockRewards distributes collected fees and block provisions in PoolFeeCollector
// across active bonded validators and their delegators proportionally to voting power.
func (k *Keeper) DistributeBlockRewards() (*big.Int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	poolBal := k.bank.GetBalance(bank.PoolFeeCollector)
	if poolBal.Sign() <= 0 {
		return big.NewInt(0), nil
	}

	activeVals := k.getActiveValidatorsSortedLocked()
	if len(activeVals) == 0 {
		return big.NewInt(0), nil
	}

	totalBonded := big.NewInt(0)
	for _, v := range activeVals {
		totalBonded.Add(totalBonded, v.Tokens)
	}
	if totalBonded.Sign() <= 0 {
		return big.NewInt(0), nil
	}

	distributedTotal := big.NewInt(0)
	for _, v := range activeVals {
		// Validator reward = (poolBal * v.Tokens) / totalBonded
		valReward := new(big.Int).Mul(poolBal, v.Tokens)
		valReward.Div(valReward, totalBonded)
		if valReward.Sign() <= 0 {
			continue
		}

		// Commission cut = (valReward * Commission.RateBps) / 10000
		commission := new(big.Int).Mul(valReward, big.NewInt(int64(v.Commission.RateBps)))
		commission.Div(commission, big.NewInt(10000))
		delegatorPool := new(big.Int).Sub(valReward, commission)

		v.AccumulatedCommission.Add(v.AccumulatedCommission, commission)
		v.AccumulatedRewards.Add(v.AccumulatedRewards, delegatorPool)

		// Allocate delegatorPool to each delegator weighted by Shares * (10000 + ActivityBoostBps)
		// SEC Classification Mitigation: Staking boosters are tied directly to active contributions
		// (clean energy generation, crop submission, governance voting) rather than passive capital alone.
		if v.DelegatorShares.Sign() > 0 && delegatorPool.Sign() > 0 {
			totalWeightedShares := big.NewInt(0)
			for _, del := range k.delegations {
				if del.ValidatorAddress == v.OperatorAddress && del.Shares.Sign() > 0 {
					mult := int64(10000 + del.ActivityBoostBps)
					wShare := new(big.Int).Mul(del.Shares, big.NewInt(mult))
					totalWeightedShares.Add(totalWeightedShares, wShare)
				}
			}
			if totalWeightedShares.Sign() > 0 {
				for _, del := range k.delegations {
					if del.ValidatorAddress == v.OperatorAddress && del.Shares.Sign() > 0 {
						mult := int64(10000 + del.ActivityBoostBps)
						wShare := new(big.Int).Mul(del.Shares, big.NewInt(mult))
						delShareReward := new(big.Int).Mul(delegatorPool, wShare)
						delShareReward.Div(delShareReward, totalWeightedShares)
						del.PendingRewards.Add(del.PendingRewards, delShareReward)
					}
				}
			}
		}
		distributedTotal.Add(distributedTotal, valReward)
	}

	return distributedTotal, nil
}

// RecordActiveContribution records a verified active contribution (e.g., CLEAN_ENERGY, CROP_SUBMISSION, GOVERNANCE_VOTE)
// and increases the delegator's ActivityBoostBps (up to +2500 bps / +25% booster) for SEC classification mitigation.
func (k *Keeper) RecordActiveContribution(delegatorAddr, validatorAddr, activityType, proofHash string) (uint64, error) {
	delegatorAddr = bank.NormalizeAddress(delegatorAddr)
	validatorAddr = bank.NormalizeAddress(validatorAddr)
	if len(proofHash) < 16 || activityType == "" {
		return 0, errors.New("active contribution requires a valid activityType and cryptographic proofHash")
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	dk := delKey(delegatorAddr, validatorAddr)
	del, exists := k.delegations[dk]
	if !exists {
		return 0, errors.New("delegation not found for active contribution booster")
	}

	del.VerifiedContributions++
	del.LastActivityProofHash = proofHash
	newBoost := del.ActivityBoostBps + 500
	if newBoost > 2500 {
		newBoost = 2500
	}
	del.ActivityBoostBps = newBoost
	return del.ActivityBoostBps, nil
}

// WithdrawDelegatorRewards transfers a delegator's accumulated rewards from PoolFeeCollector to their account.
func (k *Keeper) WithdrawDelegatorRewards(delegatorAddr, validatorAddr string) (*big.Int, error) {
	delegatorAddr = bank.NormalizeAddress(delegatorAddr)
	validatorAddr = bank.NormalizeAddress(validatorAddr)

	k.mu.Lock()
	defer k.mu.Unlock()

	dk := delKey(delegatorAddr, validatorAddr)
	del, exists := k.delegations[dk]
	if !exists || del.PendingRewards.Sign() <= 0 {
		return big.NewInt(0), nil
	}

	reward := new(big.Int).Set(del.PendingRewards)
	if err := k.bank.Transfer(bank.PoolFeeCollector, delegatorAddr, reward); err != nil {
		return nil, err
	}
	del.PendingRewards = big.NewInt(0)
	return reward, nil
}

// SlashValidator slashes a validator's tokens and unbonding entries by slashFactorBps,
// burns the slashed tokens from the protocol pool, and jails (and optionally tombstones) the validator.
func (k *Keeper) SlashValidator(consAddr string, slashFactorBps uint64, jail bool, tombstone bool) (*big.Int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	opAddr, ok := k.consAddrIndex[consAddr]
	if !ok {
		return nil, ErrValidatorNotFound
	}
	val := k.validators[opAddr]
	if val.Tombstoned {
		return big.NewInt(0), nil
	}

	slashAmount := new(big.Int).Mul(val.Tokens, big.NewInt(int64(slashFactorBps)))
	slashAmount.Div(slashAmount, big.NewInt(10000))
	if slashAmount.Cmp(val.Tokens) > 0 {
		slashAmount = new(big.Int).Set(val.Tokens)
	}

	pool := bank.PoolBondedTokens
	if val.Status != BondStatusBonded {
		pool = bank.PoolNotBondedTokens
	}
	if slashAmount.Sign() > 0 {
		if err := k.bank.BurnTokens(pool, slashAmount); err != nil {
			return nil, err
		}
		val.Tokens.Sub(val.Tokens, slashAmount)
	}

	// Also slash unbonding entries for this validator
	totalSlashed := new(big.Int).Set(slashAmount)
	for _, ubd := range k.unbondings {
		if ubd.ValidatorAddress == opAddr {
			for i := range ubd.Entries {
				entrySlash := new(big.Int).Mul(ubd.Entries[i].InitialBalance, big.NewInt(int64(slashFactorBps)))
				entrySlash.Div(entrySlash, big.NewInt(10000))
				if entrySlash.Cmp(ubd.Entries[i].Balance) > 0 {
					entrySlash = new(big.Int).Set(ubd.Entries[i].Balance)
				}
				if entrySlash.Sign() > 0 {
					_ = k.bank.BurnTokens(bank.PoolNotBondedTokens, entrySlash)
					ubd.Entries[i].Balance.Sub(ubd.Entries[i].Balance, entrySlash)
					totalSlashed.Add(totalSlashed, entrySlash)
				}
			}
		}
	}

	val.SlashCount++
	if tombstone {
		val.Tombstoned = true
	}
	if jail || tombstone {
		k.jailValidatorLocked(val)
	}

	k.recomputeActiveValidatorSetLocked()
	return totalSlashed, nil
}

func (k *Keeper) jailValidatorLocked(val *Validator) {
	if val.Jailed {
		return
	}
	val.Jailed = true
	if val.Status == BondStatusBonded && val.Tokens.Sign() > 0 {
		_ = k.bank.Transfer(bank.PoolBondedTokens, bank.PoolNotBondedTokens, val.Tokens)
		val.Status = BondStatusUnbonding
	}
}

// UnjailValidator unjails a validator if not tombstoned and self-delegation meets MinSelfDelegation.
func (k *Keeper) UnjailValidator(operatorAddr string) error {
	operatorAddr = bank.NormalizeAddress(operatorAddr)

	k.mu.Lock()
	defer k.mu.Unlock()

	val, ok := k.validators[operatorAddr]
	if !ok {
		return ErrValidatorNotFound
	}
	if val.Tombstoned {
		return ErrValidatorTombstoned
	}
	if !val.Jailed {
		return errors.New("validator is not jailed")
	}

	// Verify self-delegation >= MinSelfDelegation
	selfDel, exists := k.delegations[delKey(operatorAddr, operatorAddr)]
	if !exists {
		return ErrBelowMinSelfDelegation
	}
	selfTokens := val.TokensFromShares(selfDel.Shares)
	if selfTokens.Cmp(val.MinSelfDelegation) < 0 {
		return ErrBelowMinSelfDelegation
	}

	val.Jailed = false
	k.recomputeActiveValidatorSetLocked()
	return nil
}

func (k *Keeper) recomputeActiveValidatorSetLocked() {
	all := make([]*Validator, 0, len(k.validators))
	for _, v := range k.validators {
		all = append(all, v)
	}

	// Sort candidates deterministically: non-jailed & non-tombstoned first, then by Tokens DESC, then OperatorAddress ASC
	sort.Slice(all, func(i, j int) bool {
		eligI := !all[i].Jailed && !all[i].Tombstoned && all[i].Tokens.Cmp(bank.OneBGOV()) >= 0
		eligJ := !all[j].Jailed && !all[j].Tombstoned && all[j].Tokens.Cmp(bank.OneBGOV()) >= 0
		if eligI != eligJ {
			return eligI
		}
		cmp := all[i].Tokens.Cmp(all[j].Tokens)
		if cmp != 0 {
			return cmp > 0
		}
		return all[i].OperatorAddress < all[j].OperatorAddress
	})

	activeCount := uint32(0)
	for _, v := range all {
		eligible := !v.Jailed && !v.Tombstoned && v.Tokens.Cmp(bank.OneBGOV()) >= 0
		// MaxValidators == 0 or UnlimitedValidators == true allows an UNLIMITED number of active validators
		withinCap := k.params.UnlimitedValidators || k.params.MaxValidators == 0 || activeCount < k.params.MaxValidators
		if eligible && withinCap {
			if v.Status != BondStatusBonded {
				if v.Tokens.Sign() > 0 {
					_ = k.bank.Transfer(bank.PoolNotBondedTokens, bank.PoolBondedTokens, v.Tokens)
				}
				v.Status = BondStatusBonded
			}
			activeCount++
		} else {
			if v.Status == BondStatusBonded {
				if v.Tokens.Sign() > 0 {
					_ = k.bank.Transfer(bank.PoolBondedTokens, bank.PoolNotBondedTokens, v.Tokens)
				}
				v.Status = BondStatusUnbonding
			}
		}
	}
}

func (k *Keeper) getActiveValidatorsSortedLocked() []*Validator {
	active := make([]*Validator, 0, len(k.validators))
	for _, v := range k.validators {
		if v.Status == BondStatusBonded && !v.Jailed && !v.Tombstoned {
			active = append(active, v)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		cmp := active[i].Tokens.Cmp(active[j].Tokens)
		if cmp != 0 {
			return cmp > 0
		}
		return active[i].OperatorAddress < active[j].OperatorAddress
	})
	return active
}

// GetValidatorUpdates returns CometBFT-compatible consensus validator set updates sorted by voting power.
func (k *Keeper) GetValidatorUpdates() []ValidatorUpdate {
	k.mu.RLock()
	defer k.mu.RUnlock()

	active := k.getActiveValidatorsSortedLocked()
	updates := make([]ValidatorUpdate, 0, len(active))
	for _, v := range active {
		updates = append(updates, ValidatorUpdate{
			OperatorAddress:    v.OperatorAddress,
			ConsensusAddress:   v.ConsensusAddress,
			ConsensusPubKeyHex: v.ConsensusPubKeyHex,
			Role:               v.Role,
			VotingPower:        v.VotingPower(),
		})
	}
	return updates
}

func (k *Keeper) GetTotalBondedTokens() *big.Int {
	k.mu.RLock()
	defer k.mu.RUnlock()

	total := big.NewInt(0)
	for _, v := range k.validators {
		if v.Status == BondStatusBonded && !v.Jailed && !v.Tombstoned {
			total.Add(total, v.Tokens)
		}
	}
	return total
}

// GetAccountVotingPower calculates an address's total bonded voting power (in abgov) across all active validators.
func (k *Keeper) GetAccountVotingPower(addr string) *big.Int {
	addr = bank.NormalizeAddress(addr)
	k.mu.RLock()
	defer k.mu.RUnlock()

	power := big.NewInt(0)
	for _, del := range k.delegations {
		if del.DelegatorAddress == addr && del.Shares.Sign() > 0 {
			if val, ok := k.validators[del.ValidatorAddress]; ok && val.Status == BondStatusBonded && !val.Jailed && !val.Tombstoned {
				power.Add(power, val.TokensFromShares(del.Shares))
			}
		}
	}
	return power
}

func (k *Keeper) GetValidator(operatorAddr string) (*Validator, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	v, ok := k.validators[bank.NormalizeAddress(operatorAddr)]
	if !ok {
		return nil, ErrValidatorNotFound
	}
	return cloneValidator(v), nil
}

func (k *Keeper) GetValidatorByConsAddr(consAddr string) (*Validator, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	opAddr, ok := k.consAddrIndex[consAddr]
	if !ok {
		return nil, ErrValidatorNotFound
	}
	return cloneValidator(k.validators[opAddr]), nil
}

func (k *Keeper) GetAllValidators() []*Validator {
	k.mu.RLock()
	defer k.mu.RUnlock()

	out := make([]*Validator, 0, len(k.validators))
	for _, v := range k.validators {
		out = append(out, cloneValidator(v))
	}
	sort.Slice(out, func(i, j int) bool {
		cmp := out[i].Tokens.Cmp(out[j].Tokens)
		if cmp != 0 {
			return cmp > 0
		}
		return out[i].OperatorAddress < out[j].OperatorAddress
	})
	return out
}

func (k *Keeper) GetDelegationsForAddress(delegatorAddr string) []Delegation {
	delegatorAddr = bank.NormalizeAddress(delegatorAddr)
	k.mu.RLock()
	defer k.mu.RUnlock()

	out := []Delegation{}
	for _, d := range k.delegations {
		if d.DelegatorAddress == delegatorAddr {
			out = append(out, Delegation{
				DelegatorAddress: d.DelegatorAddress,
				ValidatorAddress: d.ValidatorAddress,
				Shares:           new(big.Int).Set(d.Shares),
				PendingRewards:   new(big.Int).Set(d.PendingRewards),
			})
		}
	}
	return out
}

func cloneValidator(v *Validator) *Validator {
	c := *v
	c.Tokens = new(big.Int).Set(v.Tokens)
	c.DelegatorShares = new(big.Int).Set(v.DelegatorShares)
	c.MinSelfDelegation = new(big.Int).Set(v.MinSelfDelegation)
	c.AccumulatedCommission = new(big.Int).Set(v.AccumulatedCommission)
	c.AccumulatedRewards = new(big.Int).Set(v.AccumulatedRewards)
	return &c
}

func (k *Keeper) StateRoot() string {
	vals := k.GetAllValidators()
	payload, _ := json.Marshal(vals)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

