# BETTERGOV On-Chain Governance (`x/gov`)

## 1. Proposal Types
1. `TEXT`: Ratifies non-binding or constitutional policy resolutions.
2. `PARAMETER_CHANGE`: Modifies consensus-critical economic or staking parameters (`mint.inflation_rate_bps`, `mint.community_tax_bps`, `staking.max_validators`, `staking.min_commission_rate_bps`).
3. `TREASURY_SPEND`: Disburses `abgov` from `PoolTreasury` (`bgov1pool_community_treasury_00000000000`) to an audited civic project recipient.
4. `SOFTWARE_UPGRADE`: Schedules a coordinated binary upgrade plan at a target `UpgradeHeight`.

## 2. Voting Power, Quorum, Threshold & Veto Rules
- **Voting Power**: Derived directly from protocol bonded stake (`stakingKeeper.GetAccountVotingPower(voter)`). Re-verified at tally time to prevent flash-unbond vote manipulation.
- **Quorum (`33.40%` / `3340 bps`)**: Minimum fraction of total network bonded stake that must participate.
- **Approval Threshold (`>50.00%` / `5000 bps`)**: Share of non-abstain votes (`Yes / (Yes + No + NoWithVeto)`) required to pass.
- **NoWithVeto (`>=33.40%` / `3340 bps`)**: If `NoWithVeto` reaches 33.4% of total votes cast, the proposal is rejected with status `PROPOSAL_STATUS_VETOED` and its proposal deposit is **burned** from `PoolGovDeposit`.

