package mint

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/bettergov/chain/x/bank"
)

// EconomicParams defines configurable emission, inflation, and reward distribution parameters.
// Final mainnet tokenomics are intentionally configurable via genesis and governance proposals.
type EconomicParams struct {
	InflationRateBps       uint64 `json:"inflation_rate_bps"`        // Current annual inflation in basis points (e.g. 400 = 4.00%)
	InflationMinBps        uint64 `json:"inflation_min_bps"`         // Minimum annual inflation (e.g. 100 = 1.00%)
	InflationMaxBps        uint64 `json:"inflation_max_bps"`         // Maximum annual inflation (e.g. 800 = 8.00%)
	InflationRateChangeBps uint64 `json:"inflation_rate_change_bps"` // Max annual rate of change (e.g. 300 = 3.00%)
	GoalBondedBps          uint64 `json:"goal_bonded_bps"`           // Target bonded stake ratio (e.g. 6700 = 67.00%)
	BlocksPerYear          uint64 `json:"blocks_per_year"`           // Expected blocks per year (e.g. 6,311,520 at 5s blocks)
	CommunityTaxBps        uint64 `json:"community_tax_bps"`         // Share of rewards allocated to public treasury (e.g. 1000 = 10.00%)
	BaseProposerRewardBps  uint64 `json:"base_proposer_reward_bps"`  // Base reward for block proposer (e.g. 100 = 1.00%)
	BonusProposerRewardBps uint64 `json:"bonus_proposer_reward_bps"` // Max bonus reward for precommit inclusion (e.g. 400 = 4.00%)
}

func DefaultEconomicParams() EconomicParams {
	return EconomicParams{
		InflationRateBps:       400,
		InflationMinBps:        100,
		InflationMaxBps:        800,
		InflationRateChangeBps: 300,
		GoalBondedBps:          6700,
		BlocksPerYear:          6311520,
		CommunityTaxBps:        1000,
		BaseProposerRewardBps:  100,
		BonusProposerRewardBps: 400,
	}
}

func (p EconomicParams) Validate() error {
	if p.InflationMinBps > p.InflationMaxBps {
		return fmt.Errorf("inflation_min_bps (%d) cannot exceed inflation_max_bps (%d)", p.InflationMinBps, p.InflationMaxBps)
	}
	if p.InflationRateBps < p.InflationMinBps || p.InflationRateBps > p.InflationMaxBps {
		return fmt.Errorf("inflation_rate_bps (%d) outside [min=%d, max=%d]", p.InflationRateBps, p.InflationMinBps, p.InflationMaxBps)
	}
	if p.GoalBondedBps == 0 || p.GoalBondedBps > 10000 {
		return fmt.Errorf("goal_bonded_bps (%d) must be in (0, 10000]", p.GoalBondedBps)
	}
	if p.BlocksPerYear == 0 {
		return fmt.Errorf("blocks_per_year must be positive")
	}
	if p.CommunityTaxBps+p.BaseProposerRewardBps+p.BonusProposerRewardBps > 10000 {
		return fmt.Errorf("combined tax and proposer rewards exceed 10000 bps")
	}
	return nil
}

type BlockProvisionBreakdown struct {
	TotalMinted       *big.Int `json:"total_minted"`
	TreasuryAllocation *big.Int `json:"treasury_allocation"`
	ValidatorPool     *big.Int `json:"validator_pool"`
	EffectiveInflationBps uint64 `json:"effective_inflation_bps"`
}

type Keeper struct {
	mu     sync.RWMutex
	params EconomicParams
	bank   *bank.Keeper
}

func NewKeeper(b *bank.Keeper, params EconomicParams) *Keeper {
	return &Keeper{
		params: params,
		bank:   b,
	}
}

func (k *Keeper) GetParams() EconomicParams {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.params
}

func (k *Keeper) SetParams(p EconomicParams) error {
	if err := p.Validate(); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.params = p
	return nil
}

// NextInflationRateBps computes the dynamic inflation adjustment based on current bonded ratio vs GoalBondedBps.
func (k *Keeper) NextInflationRateBps(bondedTokens *big.Int) uint64 {
	k.mu.RLock()
	p := k.params
	k.mu.RUnlock()

	totalSupply := k.bank.GetTotalSupply()
	if totalSupply.Sign() <= 0 {
		return p.InflationRateBps
	}

	// bondedRatioBps = (bondedTokens * 10000) / totalSupply
	bondedRatioBps := new(big.Int).Mul(bondedTokens, big.NewInt(10000))
	bondedRatioBps.Div(bondedRatioBps, totalSupply)
	currentBonded := bondedRatioBps.Uint64()

	currentRate := int64(p.InflationRateBps)
	// Adjust by 1 bps per block audit cycle if under/over GoalBondedBps, bounded by [InflationMinBps, InflationMaxBps]
	if currentBonded < p.GoalBondedBps {
		currentRate += 1
	} else if currentBonded > p.GoalBondedBps {
		currentRate -= 1
	}
	if currentRate < int64(p.InflationMinBps) {
		currentRate = int64(p.InflationMinBps)
	}
	if currentRate > int64(p.InflationMaxBps) {
		currentRate = int64(p.InflationMaxBps)
	}
	return uint64(currentRate)
}

// MintBlockProvision mints the per-block BGOV provision and splits it between the Community Treasury and FeeCollector (for validators/delegators).
func (k *Keeper) MintBlockProvision(bondedTokens *big.Int) (BlockProvisionBreakdown, error) {
	nextRate := k.NextInflationRateBps(bondedTokens)

	k.mu.Lock()
	k.params.InflationRateBps = nextRate
	p := k.params
	k.mu.Unlock()

	totalSupply := k.bank.GetTotalSupply()
	if totalSupply.Sign() <= 0 {
		return BlockProvisionBreakdown{
			TotalMinted:           big.NewInt(0),
			TreasuryAllocation:    big.NewInt(0),
			ValidatorPool:         big.NewInt(0),
			EffectiveInflationBps: nextRate,
		}, nil
	}

	// blockProvision = (totalSupply * InflationRateBps) / (10000 * BlocksPerYear)
	numerator := new(big.Int).Mul(totalSupply, big.NewInt(int64(p.InflationRateBps)))
	denominator := new(big.Int).Mul(big.NewInt(10000), big.NewInt(int64(p.BlocksPerYear)))
	provision := new(big.Int).Div(numerator, denominator)

	if provision.Sign() <= 0 {
		return BlockProvisionBreakdown{
			TotalMinted:           big.NewInt(0),
			TreasuryAllocation:    big.NewInt(0),
			ValidatorPool:         big.NewInt(0),
			EffectiveInflationBps: nextRate,
		}, nil
	}

	// Treasury cut = provision * CommunityTaxBps / 10000
	treasuryCut := new(big.Int).Mul(provision, big.NewInt(int64(p.CommunityTaxBps)))
	treasuryCut.Div(treasuryCut, big.NewInt(10000))
	validatorCut := new(big.Int).Sub(provision, treasuryCut)

	if treasuryCut.Sign() > 0 {
		if err := k.bank.MintTokens(bank.PoolTreasury, treasuryCut); err != nil {
			return BlockProvisionBreakdown{}, err
		}
	}
	if validatorCut.Sign() > 0 {
		if err := k.bank.MintTokens(bank.PoolFeeCollector, validatorCut); err != nil {
			return BlockProvisionBreakdown{}, err
		}
	}

	return BlockProvisionBreakdown{
		TotalMinted:           provision,
		TreasuryAllocation:    treasuryCut,
		ValidatorPool:         validatorCut,
		EffectiveInflationBps: nextRate,
	}, nil
}

