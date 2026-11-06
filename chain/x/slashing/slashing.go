package slashing

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

	"github.com/bettergov/chain/x/staking"
)

var (
	ErrStillJailed           = errors.New("validator jail period has not expired")
	ErrDuplicateDoubleSign   = errors.New("double-sign evidence already processed")
	ErrInvalidDoubleSignEvid = errors.New("invalid double-sign evidence: conflicting block hashes must differ at same height/round")
)

type SlashingParams struct {
	SignedBlocksWindow         int64  `json:"signed_blocks_window"`           // Window size for downtime tracking (e.g. 100 blocks)
	MinSignedPerWindowBps      uint64 `json:"min_signed_per_window_bps"`      // Minimum blocks signed in bps (e.g. 5000 = 50%)
	DowntimeJailDurationSec    int64  `json:"downtime_jail_duration_seconds"` // Duration jailed for downtime (e.g. 600s)
	SlashFractionDoubleSignBps uint64 `json:"slash_fraction_double_sign_bps"` // Slash % for double signing (e.g. 500 = 5.00%)
	SlashFractionDowntimeBps   uint64 `json:"slash_fraction_downtime_bps"`    // Slash % for downtime (e.g. 50 = 0.50%)
}

func DefaultParams() SlashingParams {
	return SlashingParams{
		SignedBlocksWindow:         100,
		MinSignedPerWindowBps:      5000, // Must sign at least 50% of window
		DowntimeJailDurationSec:    600,  // 10 minutes jail
		SlashFractionDoubleSignBps: 500,  // 5.00% slash + permanent tombstone
		SlashFractionDowntimeBps:   50,   // 0.50% slash + temporary jail
	}
}

type ValidatorSigningInfo struct {
	ConsensusAddress    string        `json:"consensus_address"`
	StartHeight         int64         `json:"start_height"`
	IndexOffset         int64         `json:"index_offset"`
	JailedUntil         time.Time     `json:"jailed_until"`
	Tombstoned          bool          `json:"tombstoned"`
	MissedBlocksCounter int64         `json:"missed_blocks_counter"`
	MissedBlocksBitArr  map[int64]bool `json:"missed_blocks_bit_arr"`
}

type DoubleSignEvidence struct {
	ConsensusAddress string    `json:"consensus_address"`
	Height           int64     `json:"height"`
	Round            int32     `json:"round"`
	BlockHashA       string    `json:"block_hash_a"`
	BlockHashB       string    `json:"block_hash_b"`
	EvidenceTime     time.Time `json:"evidence_time"`
}

func (e DoubleSignEvidence) Hash() string {
	raw, _ := json.Marshal(e)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type SlashEvent struct {
	ConsensusAddress string    `json:"consensus_address"`
	OperatorAddress  string    `json:"operator_address"`
	Reason           string    `json:"reason"`
	Height           int64     `json:"height"`
	SlashedAmount    string    `json:"slashed_amount"`
	Tombstoned       bool      `json:"tombstoned"`
	Timestamp        time.Time `json:"timestamp"`
}

type Keeper struct {
	mu                sync.RWMutex
	params            SlashingParams
	staking           *staking.Keeper
	signingInfos      map[string]*ValidatorSigningInfo
	processedEvidence map[string]bool
	slashEvents       []SlashEvent
}

func NewKeeper(sk *staking.Keeper, params SlashingParams) *Keeper {
	return &Keeper{
		params:            params,
		staking:           sk,
		signingInfos:      make(map[string]*ValidatorSigningInfo),
		processedEvidence: make(map[string]bool),
		slashEvents:       []SlashEvent{},
	}
}

func (k *Keeper) GetParams() SlashingParams {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.params
}

func (k *Keeper) EnsureSigningInfo(consAddr string, startHeight int64) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.signingInfos[consAddr]; !ok {
		k.signingInfos[consAddr] = &ValidatorSigningInfo{
			ConsensusAddress:   consAddr,
			StartHeight:        startHeight,
			MissedBlocksBitArr: make(map[int64]bool),
		}
	}
}

// HandleValidatorSignature tracks validator liveness per block and slashes/jails for downtime if missed blocks exceed maxMissed.
func (k *Keeper) HandleValidatorSignature(consAddr string, signed bool, height int64, blockTime time.Time) (*SlashEvent, error) {
	k.EnsureSigningInfo(consAddr, height)

	k.mu.Lock()
	defer k.mu.Unlock()

	info := k.signingInfos[consAddr]
	if info.Tombstoned {
		return nil, nil
	}

	window := k.params.SignedBlocksWindow
	index := info.IndexOffset % window
	info.IndexOffset++

	previousMissed := info.MissedBlocksBitArr[index]
	missed := !signed

	switch {
	case !previousMissed && missed:
		info.MissedBlocksBitArr[index] = true
		info.MissedBlocksCounter++
	case previousMissed && !missed:
		info.MissedBlocksBitArr[index] = false
		info.MissedBlocksCounter--
	}

	minSigned := (window * int64(k.params.MinSignedPerWindowBps)) / 10000
	maxMissed := window - minSigned

	if info.MissedBlocksCounter > maxMissed {
		val, err := k.staking.GetValidatorByConsAddr(consAddr)
		if err != nil {
			return nil, err
		}
		slashedAmt, err := k.staking.SlashValidator(consAddr, k.params.SlashFractionDowntimeBps, true, false)
		if err != nil {
			return nil, err
		}
		info.JailedUntil = blockTime.Add(time.Duration(k.params.DowntimeJailDurationSec) * time.Second)
		// Reset missed blocks window after downtime slash
		info.MissedBlocksBitArr = make(map[int64]bool)
		info.MissedBlocksCounter = 0
		info.IndexOffset = 0

		ev := SlashEvent{
			ConsensusAddress: consAddr,
			OperatorAddress:  val.OperatorAddress,
			Reason:           "downtime_exceeded_window",
			Height:           height,
			SlashedAmount:    slashedAmt.String(),
			Tombstoned:       false,
			Timestamp:        blockTime,
		}
		k.slashEvents = append(k.slashEvents, ev)
		return &ev, nil
	}
	return nil, nil
}

// HandleDoubleSignEvidence verifies equivocation evidence, slashes by SlashFractionDoubleSignBps, and permanently tombstones the validator.
func (k *Keeper) HandleDoubleSignEvidence(ev DoubleSignEvidence, currentHeight int64, blockTime time.Time) (*SlashEvent, error) {
	if ev.BlockHashA == "" || ev.BlockHashB == "" || ev.BlockHashA == ev.BlockHashB || ev.Height <= 0 {
		return nil, ErrInvalidDoubleSignEvid
	}
	evHash := ev.Hash()

	k.EnsureSigningInfo(ev.ConsensusAddress, currentHeight)

	k.mu.Lock()
	defer k.mu.Unlock()

	if k.processedEvidence[evHash] {
		return nil, ErrDuplicateDoubleSign
	}
	info := k.signingInfos[ev.ConsensusAddress]
	if info.Tombstoned {
		return nil, staking.ErrValidatorTombstoned
	}

	val, err := k.staking.GetValidatorByConsAddr(ev.ConsensusAddress)
	if err != nil {
		return nil, err
	}

	slashedAmt, err := k.staking.SlashValidator(ev.ConsensusAddress, k.params.SlashFractionDoubleSignBps, true, true)
	if err != nil {
		return nil, err
	}

	info.Tombstoned = true
	info.JailedUntil = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	k.processedEvidence[evHash] = true

	slashEv := SlashEvent{
		ConsensusAddress: ev.ConsensusAddress,
		OperatorAddress:  val.OperatorAddress,
		Reason:           fmt.Sprintf("double_sign_at_height_%d_round_%d", ev.Height, ev.Round),
		Height:           currentHeight,
		SlashedAmount:    slashedAmt.String(),
		Tombstoned:       true,
		Timestamp:        blockTime,
	}
	k.slashEvents = append(k.slashEvents, slashEv)
	return &slashEv, nil
}

// Unjail verifies that the validator's jail duration has elapsed and they are not tombstoned, then unjails in x/staking.
func (k *Keeper) Unjail(operatorAddr string, blockTime time.Time) error {
	val, err := k.staking.GetValidator(operatorAddr)
	if err != nil {
		return err
	}

	k.mu.RLock()
	info, exists := k.signingInfos[val.ConsensusAddress]
	k.mu.RUnlock()

	if exists {
		if info.Tombstoned {
			return staking.ErrValidatorTombstoned
		}
		if blockTime.Before(info.JailedUntil) {
			return ErrStillJailed
		}
	}

	return k.staking.UnjailValidator(operatorAddr)
}

func (k *Keeper) GetSigningInfo(consAddr string) (*ValidatorSigningInfo, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	info, ok := k.signingInfos[consAddr]
	if !ok {
		return nil, false
	}
	c := *info
	return &c, true
}

func (k *Keeper) GetSlashEvents() []SlashEvent {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]SlashEvent, len(k.slashEvents))
	copy(out, k.slashEvents)
	return out
}

func (k *Keeper) StateRoot() string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	keys := make([]string, 0, len(k.signingInfos))
	for addr := range k.signingInfos {
		keys = append(keys, addr)
	}
	sort.Strings(keys)

	infos := make([]*ValidatorSigningInfo, 0, len(keys))
	for _, key := range keys {
		infos = append(infos, k.signingInfos[key])
	}
	payload, _ := json.Marshal(struct {
		Infos  []*ValidatorSigningInfo `json:"infos"`
		Events []SlashEvent            `json:"events"`
	}{
		Infos:  infos,
		Events: k.slashEvents,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// Helper for unused big import check
var _ = big.NewInt

