package gov

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
	"github.com/bettergov/chain/x/staking"
)

type ProposalType string

const (
	ProposalTypeText            ProposalType = "TEXT"
	ProposalTypeParameterChange ProposalType = "PARAMETER_CHANGE"
	ProposalTypeTreasurySpend   ProposalType = "TREASURY_SPEND"
	ProposalTypeSoftwareUpgrade ProposalType = "SOFTWARE_UPGRADE"
)

type ProposalStatus string

const (
	StatusDepositPeriod ProposalStatus = "PROPOSAL_STATUS_DEPOSIT_PERIOD"
	StatusVotingPeriod  ProposalStatus = "PROPOSAL_STATUS_VOTING_PERIOD"
	StatusPassed        ProposalStatus = "PROPOSAL_STATUS_PASSED"
	StatusRejected      ProposalStatus = "PROPOSAL_STATUS_REJECTED"
	StatusVetoed        ProposalStatus = "PROPOSAL_STATUS_VETOED"
	StatusExecuted      ProposalStatus = "PROPOSAL_STATUS_EXECUTED"
)

type VoteOption string

const (
	VoteOptionYes        VoteOption = "VOTE_OPTION_YES"
	VoteOptionNo         VoteOption = "VOTE_OPTION_NO"
	VoteOptionAbstain    VoteOption = "VOTE_OPTION_ABSTAIN"
	VoteOptionNoWithVeto VoteOption = "VOTE_OPTION_NO_WITH_VETO"
)

var (
	ErrProposalNotFound    = errors.New("governance proposal not found")
	ErrNotInDepositPeriod  = errors.New("proposal is not in deposit period")
	ErrNotInVotingPeriod   = errors.New("proposal is not in voting period")
	ErrInvalidVoteOption   = errors.New("invalid vote option")
	ErrZeroVotingPower     = errors.New("account has zero bonded staking voting power")
	ErrInvalidProposalSpec = errors.New("invalid proposal content or parameters")
)

type GovParams struct {
	MinDepositAbgov      *big.Int `json:"min_deposit_abgov"`        // Minimum total deposit to enter voting (e.g. 1000 BGOV)
	MaxDepositPeriodSec  int64    `json:"max_deposit_period_sec"`   // Max time to reach min deposit
	VotingPeriodSec      int64    `json:"voting_period_sec"`        // Voting period duration
	QuorumBps            uint64   `json:"quorum_bps"`               // Min participation of total bonded stake (e.g. 3340 = 33.40%)
	ThresholdBps         uint64   `json:"threshold_bps"`            // Min Yes ratio of non-abstain votes (e.g. 5000 = >50.00%)
	VetoThresholdBps     uint64   `json:"veto_threshold_bps"`       // Veto ratio of total votes (e.g. 3340 = 33.40%)
}

func DefaultParams() GovParams {
	return GovParams{
		MinDepositAbgov:     bank.BGOVToAbgov(1000),
		MaxDepositPeriodSec: 172800, // 2 days
		VotingPeriodSec:     259200, // 3 days
		QuorumBps:           3340,   // 33.4% quorum
		ThresholdBps:        5000,   // >50.0% Yes approval
		VetoThresholdBps:    3340,   // 33.4% NoWithVeto burns deposit and rejects
	}
}

type ParameterChange struct {
	Subspace string `json:"subspace"`
	Key      string `json:"key"`
	Value    string `json:"value"`
}

type TreasurySpendSpec struct {
	Recipient   string   `json:"recipient"`
	AmountAbgov *big.Int `json:"amount_abgov"`
	ProjectID   string   `json:"project_id"`
}

type UpgradePlanSpec struct {
	Name          string `json:"name"`
	UpgradeHeight int64  `json:"upgrade_height"`
	InfoHash      string `json:"info_hash"`
}

type DepositRecord struct {
	Depositor string    `json:"depositor"`
	Amount    *big.Int  `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

type VoteRecord struct {
	ProposalID  uint64     `json:"proposal_id"`
	Voter       string     `json:"voter"`
	Option      VoteOption `json:"option"`
	VotingPower *big.Int   `json:"voting_power"`
	Reason      string     `json:"reason"`
	Timestamp   time.Time  `json:"timestamp"`
}

type TallyResult struct {
	YesCount        *big.Int `json:"yes_count"`
	AbstainCount    *big.Int `json:"abstain_count"`
	NoCount         *big.Int `json:"no_count"`
	NoWithVetoCount *big.Int `json:"no_with_veto_count"`
	TotalVotedPower *big.Int `json:"total_voted_power"`
	TotalBonded     *big.Int `json:"total_bonded"`
	QuorumReached   bool     `json:"quorum_reached"`
	Vetoed          bool     `json:"vetoed"`
	Passed          bool     `json:"passed"`
}

type Proposal struct {
	ID               uint64            `json:"id"`
	Title            string            `json:"title"`
	Summary          string            `json:"summary"`
	MetadataURI      string            `json:"metadata_uri"`
	Proposer         string            `json:"proposer"`
	ProposalType     ProposalType      `json:"proposal_type"`
	Status           ProposalStatus    `json:"status"`
	SubmitTime       time.Time         `json:"submit_time"`
	DepositEndTime   time.Time         `json:"deposit_end_time"`
	VotingStartTime  time.Time         `json:"voting_start_time"`
	VotingEndTime    time.Time         `json:"voting_end_time"`
	TotalDeposit     *big.Int          `json:"total_deposit"`
	Deposits         []DepositRecord   `json:"deposits"`
	ParamChanges     []ParameterChange `json:"param_changes,omitempty"`
	TreasurySpend    *TreasurySpendSpec `json:"treasury_spend,omitempty"`
	UpgradePlan      *UpgradePlanSpec  `json:"upgrade_plan,omitempty"`
	FinalTallyResult *TallyResult      `json:"final_tally_result,omitempty"`
	ExecutionLog     string            `json:"execution_log,omitempty"`
}

type ParamChangeHook func(changes []ParameterChange) error

type Keeper struct {
	mu               sync.RWMutex
	params           GovParams
	bank             *bank.Keeper
	staking          *staking.Keeper
	nextProposalID   uint64
	proposals        map[uint64]*Proposal
	votes            map[uint64]map[string]VoteRecord
	scheduledUpgrade *UpgradePlanSpec
	paramHook        ParamChangeHook
}

func NewKeeper(b *bank.Keeper, sk *staking.Keeper, params GovParams) *Keeper {
	return &Keeper{
		params:         params,
		bank:           b,
		staking:        sk,
		nextProposalID: 1,
		proposals:      make(map[uint64]*Proposal),
		votes:          make(map[uint64]map[string]VoteRecord),
	}
}

func (k *Keeper) SetParamChangeHook(hook ParamChangeHook) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.paramHook = hook
}

func (k *Keeper) GetParams() GovParams {
	k.mu.RLock()
	defer k.mu.RUnlock()
	p := k.params
	p.MinDepositAbgov = new(big.Int).Set(k.params.MinDepositAbgov)
	return p
}

func (k *Keeper) SubmitProposal(
	proposer string,
	title string,
	summary string,
	metadataURI string,
	pType ProposalType,
	initialDeposit *big.Int,
	paramChanges []ParameterChange,
	treasurySpend *TreasurySpendSpec,
	upgradePlan *UpgradePlanSpec,
	blockTime time.Time,
) (*Proposal, error) {
	proposer = bank.NormalizeAddress(proposer)
	if title == "" || summary == "" {
		return nil, ErrInvalidProposalSpec
	}
	if pType == ProposalTypeParameterChange && len(paramChanges) == 0 {
		return nil, ErrInvalidProposalSpec
	}
	if pType == ProposalTypeTreasurySpend && (treasurySpend == nil || treasurySpend.Recipient == "" || treasurySpend.AmountAbgov == nil || treasurySpend.AmountAbgov.Sign() <= 0) {
		return nil, ErrInvalidProposalSpec
	}
	if pType == ProposalTypeSoftwareUpgrade && (upgradePlan == nil || upgradePlan.Name == "" || upgradePlan.UpgradeHeight <= 0) {
		return nil, ErrInvalidProposalSpec
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if initialDeposit != nil && initialDeposit.Sign() > 0 {
		if err := k.bank.Transfer(proposer, bank.PoolGovDeposit, initialDeposit); err != nil {
			return nil, err
		}
	} else {
		initialDeposit = big.NewInt(0)
	}

	id := k.nextProposalID
	k.nextProposalID++

	prop := &Proposal{
		ID:             id,
		Title:          title,
		Summary:        summary,
		MetadataURI:    metadataURI,
		Proposer:       proposer,
		ProposalType:   pType,
		Status:         StatusDepositPeriod,
		SubmitTime:     blockTime,
		DepositEndTime: blockTime.Add(time.Duration(k.params.MaxDepositPeriodSec) * time.Second),
		TotalDeposit:   new(big.Int).Set(initialDeposit),
		Deposits:       []DepositRecord{},
		ParamChanges:   paramChanges,
		TreasurySpend:  treasurySpend,
		UpgradePlan:    upgradePlan,
	}

	if initialDeposit.Sign() > 0 {
		prop.Deposits = append(prop.Deposits, DepositRecord{
			Depositor: proposer,
			Amount:    new(big.Int).Set(initialDeposit),
			Timestamp: blockTime,
		})
	}

	if prop.TotalDeposit.Cmp(k.params.MinDepositAbgov) >= 0 {
		prop.Status = StatusVotingPeriod
		prop.VotingStartTime = blockTime
		prop.VotingEndTime = blockTime.Add(time.Duration(k.params.VotingPeriodSec) * time.Second)
	}

	k.proposals[id] = prop
	k.votes[id] = make(map[string]VoteRecord)
	return prop, nil
}

func (k *Keeper) Deposit(proposalID uint64, depositor string, amount *big.Int, blockTime time.Time) (*Proposal, error) {
	if amount == nil || amount.Sign() <= 0 {
		return nil, bank.ErrInvalidAmount
	}
	depositor = bank.NormalizeAddress(depositor)

	k.mu.Lock()
	defer k.mu.Unlock()

	prop, ok := k.proposals[proposalID]
	if !ok {
		return nil, ErrProposalNotFound
	}
	if prop.Status != StatusDepositPeriod {
		return nil, ErrNotInDepositPeriod
	}

	if err := k.bank.Transfer(depositor, bank.PoolGovDeposit, amount); err != nil {
		return nil, err
	}

	prop.TotalDeposit.Add(prop.TotalDeposit, amount)
	prop.Deposits = append(prop.Deposits, DepositRecord{
		Depositor: depositor,
		Amount:    new(big.Int).Set(amount),
		Timestamp: blockTime,
	})

	if prop.TotalDeposit.Cmp(k.params.MinDepositAbgov) >= 0 {
		prop.Status = StatusVotingPeriod
		prop.VotingStartTime = blockTime
		prop.VotingEndTime = blockTime.Add(time.Duration(k.params.VotingPeriodSec) * time.Second)
	}
	return prop, nil
}

func (k *Keeper) Vote(proposalID uint64, voter string, option VoteOption, reason string, blockTime time.Time) (*VoteRecord, error) {
	voter = bank.NormalizeAddress(voter)
	switch option {
	case VoteOptionYes, VoteOptionNo, VoteOptionAbstain, VoteOptionNoWithVeto:
	default:
		return nil, ErrInvalidVoteOption
	}

	votingPower := k.staking.GetAccountVotingPower(voter)
	if votingPower.Sign() <= 0 {
		return nil, ErrZeroVotingPower
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	prop, ok := k.proposals[proposalID]
	if !ok {
		return nil, ErrProposalNotFound
	}
	if prop.Status != StatusVotingPeriod {
		return nil, ErrNotInVotingPeriod
	}

	rec := VoteRecord{
		ProposalID:  proposalID,
		Voter:       voter,
		Option:      option,
		VotingPower: new(big.Int).Set(votingPower),
		Reason:      reason,
		Timestamp:   blockTime,
	}
	k.votes[proposalID][voter] = rec
	return &rec, nil
}

// ComputeTally calculates the live or final tally for a proposal using real protocol bonded stake.
func (k *Keeper) ComputeTally(proposalID uint64) (*TallyResult, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.computeTallyLocked(proposalID)
}

func (k *Keeper) computeTallyLocked(proposalID uint64) (*TallyResult, error) {
	_, ok := k.proposals[proposalID]
	if !ok {
		return nil, ErrProposalNotFound
	}

	yes := big.NewInt(0)
	abstain := big.NewInt(0)
	no := big.NewInt(0)
	veto := big.NewInt(0)

	for _, v := range k.votes[proposalID] {
		// Re-query current bonded voting power to prevent flash-unbond vote manipulation
		livePower := k.staking.GetAccountVotingPower(v.Voter)
		if livePower.Sign() <= 0 {
			livePower = v.VotingPower
		}
		switch v.Option {
		case VoteOptionYes:
			yes.Add(yes, livePower)
		case VoteOptionAbstain:
			abstain.Add(abstain, livePower)
		case VoteOptionNo:
			no.Add(no, livePower)
		case VoteOptionNoWithVeto:
			veto.Add(veto, livePower)
		}
	}

	totalVoted := new(big.Int).Add(yes, abstain)
	totalVoted.Add(totalVoted, no)
	totalVoted.Add(totalVoted, veto)

	totalBonded := k.staking.GetTotalBondedTokens()
	res := &TallyResult{
		YesCount:        yes,
		AbstainCount:    abstain,
		NoCount:         no,
		NoWithVetoCount: veto,
		TotalVotedPower: totalVoted,
		TotalBonded:     totalBonded,
	}

	if totalBonded.Sign() <= 0 || totalVoted.Sign() <= 0 {
		return res, nil
	}

	// Check Quorum: (totalVoted * 10000) / totalBonded >= QuorumBps
	participationBps := new(big.Int).Mul(totalVoted, big.NewInt(10000))
	participationBps.Div(participationBps, totalBonded)
	if participationBps.Uint64() < k.params.QuorumBps {
		return res, nil
	}
	res.QuorumReached = true

	// Check Veto: (veto * 10000) / totalVoted >= VetoThresholdBps
	vetoBps := new(big.Int).Mul(veto, big.NewInt(10000))
	vetoBps.Div(vetoBps, totalVoted)
	if vetoBps.Uint64() >= k.params.VetoThresholdBps {
		res.Vetoed = true
		return res, nil
	}

	// Check Approval Threshold among non-abstain votes: (yes * 10000) / (yes + no + veto) > ThresholdBps
	nonAbstain := new(big.Int).Add(yes, no)
	nonAbstain.Add(nonAbstain, veto)
	if nonAbstain.Sign() <= 0 {
		return res, nil
	}
	yesBps := new(big.Int).Mul(yes, big.NewInt(10000))
	yesBps.Div(yesBps, nonAbstain)
	if yesBps.Uint64() > k.params.ThresholdBps {
		res.Passed = true
	}

	return res, nil
}

// EndBlocker processes expired deposit periods and finalizes/executes proposals whose voting period has ended.
func (k *Keeper) EndBlocker(blockTime time.Time) []uint64 {
	k.mu.Lock()
	defer k.mu.Unlock()

	finalizedIDs := []uint64{}
	ids := make([]uint64, 0, len(k.proposals))
	for id := range k.proposals {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	for _, id := range ids {
		prop := k.proposals[id]
		if prop.Status == StatusDepositPeriod && !blockTime.Before(prop.DepositEndTime) {
			// Burn expired insufficient deposits to deter spam
			if prop.TotalDeposit.Sign() > 0 {
				_ = k.bank.BurnTokens(bank.PoolGovDeposit, prop.TotalDeposit)
			}
			prop.Status = StatusRejected
			prop.ExecutionLog = "deposit period expired without reaching min_deposit"
			finalizedIDs = append(finalizedIDs, id)
			continue
		}

		if prop.Status == StatusVotingPeriod && !blockTime.Before(prop.VotingEndTime) {
			tally, _ := k.computeTallyLocked(id)
			prop.FinalTallyResult = tally

			if tally.Vetoed {
				prop.Status = StatusVetoed
				// Burn deposits on Veto
				if prop.TotalDeposit.Sign() > 0 {
					_ = k.bank.BurnTokens(bank.PoolGovDeposit, prop.TotalDeposit)
				}
				prop.ExecutionLog = "proposal vetoed by community; deposit burned"
			} else if tally.Passed {
				prop.Status = StatusPassed
				k.refundDepositsLocked(prop)
				k.executeProposalLocked(prop)
			} else {
				prop.Status = StatusRejected
				k.refundDepositsLocked(prop)
				prop.ExecutionLog = "proposal rejected (quorum or approval threshold not met); deposits refunded"
			}
			finalizedIDs = append(finalizedIDs, id)
		}
	}
	return finalizedIDs
}

func (k *Keeper) refundDepositsLocked(prop *Proposal) {
	for _, dep := range prop.Deposits {
		if dep.Amount.Sign() > 0 {
			_ = k.bank.Transfer(bank.PoolGovDeposit, dep.Depositor, dep.Amount)
		}
	}
}

func (k *Keeper) executeProposalLocked(prop *Proposal) {
	switch prop.ProposalType {
	case ProposalTypeText:
		prop.Status = StatusExecuted
		prop.ExecutionLog = "text proposal ratified on-chain"
	case ProposalTypeParameterChange:
		if k.paramHook != nil {
			if err := k.paramHook(prop.ParamChanges); err != nil {
				prop.ExecutionLog = fmt.Sprintf("parameter execution failed: %v", err)
				return
			}
		}
		prop.Status = StatusExecuted
		prop.ExecutionLog = fmt.Sprintf("applied %d parameter change(s)", len(prop.ParamChanges))
	case ProposalTypeTreasurySpend:
		if prop.TreasurySpend != nil {
			err := k.bank.Transfer(bank.PoolTreasury, prop.TreasurySpend.Recipient, prop.TreasurySpend.AmountAbgov)
			if err != nil {
				prop.ExecutionLog = fmt.Sprintf("treasury transfer failed: %v", err)
				return
			}
			prop.Status = StatusExecuted
			prop.ExecutionLog = fmt.Sprintf("disbursed %s abgov to %s for project %s",
				prop.TreasurySpend.AmountAbgov.String(), prop.TreasurySpend.Recipient, prop.TreasurySpend.ProjectID)
		}
	case ProposalTypeSoftwareUpgrade:
		if prop.UpgradePlan != nil {
			planCopy := *prop.UpgradePlan
			k.scheduledUpgrade = &planCopy
			prop.Status = StatusExecuted
			prop.ExecutionLog = fmt.Sprintf("scheduled upgrade %s at height %d", planCopy.Name, planCopy.UpgradeHeight)
		}
	}
}

func (k *Keeper) GetProposal(id uint64) (*Proposal, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	p, ok := k.proposals[id]
	if !ok {
		return nil, ErrProposalNotFound
	}
	return p, nil
}

func (k *Keeper) GetAllProposals() []*Proposal {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]*Proposal, 0, len(k.proposals))
	for _, p := range k.proposals {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (k *Keeper) GetProposalVotes(proposalID uint64) []VoteRecord {
	k.mu.RLock()
	defer k.mu.RUnlock()
	vmap, ok := k.votes[proposalID]
	if !ok {
		return []VoteRecord{}
	}
	out := make([]VoteRecord, 0, len(vmap))
	for _, v := range vmap {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Voter < out[j].Voter })
	return out
}

func (k *Keeper) GetScheduledUpgrade() *UpgradePlanSpec {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.scheduledUpgrade
}

func (k *Keeper) StateRoot() string {
	props := k.GetAllProposals()
	payload, _ := json.Marshal(props)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

