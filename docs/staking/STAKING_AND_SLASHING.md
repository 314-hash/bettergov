# BETTERGOV Protocol Staking & Slashing (`x/staking` & `x/slashing`)

## 1. Native Staking Lifecycle
BETTERGOV implements protocol-level Proof-of-Stake in Go (`chain/x/staking/staking.go`). It does **not** use an ERC-20 staking contract substitute.

- **Validator Registration (`CreateValidator`)**:
  - Requires a unique 32-byte Ed25519 consensus public key (`DeriveConsensusAddress`), operator address, moniker, commission rates (`RateBps >= MinCommissionRateBps`, `MaxRateBps`, `MaxChangeRateBps`), and `SelfBond >= MinSelfDelegation`.
- **Delegation (`Delegate`)**:
  - Transfers `abgov` from delegator to `PoolBondedTokens` and issues proportional validator shares (`shares = amount * DelegatorShares / Tokens`).
- **Redelegation (`BeginRedelegate`)**:
  - Moves stake from `ValidatorSrc` to `ValidatorDst` while enforcing **transitive redelegation lock protection** (`ErrTransitiveRedelegation` if `ValidatorSrc` is itself the destination of an active redelegation within `UnbondingTimeSeconds`).
- **Unbonding (`Undelegate`)**:
  - Moves unbonded tokens from `PoolBondedTokens` to `PoolNotBondedTokens` for `UnbondingTimeSeconds` (60s devnet / 1d testnet / 21d mainnet). If a validator's self-delegation drops below `MinSelfDelegation`, the validator is automatically jailed.

## 2. Slashing & Validator Safeguards
- **Double-Sign Equivocation (`HandleDoubleSignEvidence`)**:
  - Slashes `5.00%` (`500 bps`) of bonded tokens and unbonding entries, burns the slashed `abgov`, jails the validator until year `9999`, and sets `Tombstoned = true` permanently.
- **Downtime Liveness Violation (`HandleValidatorSignature`)**:
  - Tracks missed blocks over `SignedBlocksWindow`. If a validator misses more than `100% - MinSignedPerWindowBps` (default $>50\%$ missed), slashes `0.50%` (`50 bps`) and jails the validator for `DowntimeJailDurationSec`.
- **Unjailing (`Unjail`)**:
  - Allowed only after `JailedUntil` has elapsed, `Tombstoned == false`, and `SelfDelegation >= MinSelfDelegation`.

