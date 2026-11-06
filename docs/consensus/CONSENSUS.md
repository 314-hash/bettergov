# BETTERGOV Consensus Specification (CometBFT BFT + Native PoS)

## 1. Consensus Architecture
BETTERGOV executes a deterministic Byzantine Fault Tolerant (BFT) Proof-of-Stake state machine (`BetterGovApp` in `chain/app/app.go`):
- **Block Lifecycle (`FinalizeBlock`)**:
  1. `BeginBlock`: Mints configurable per-block provisions (`x/mint`), evaluates validator liveness signatures against the sliding `SignedBlocksWindow` (`x/slashing`), and processes equivocation evidence.
  2. `DeliverTx`: Executes native module messages and EIP-155 Ethereum transactions (`x/evm`), verifying account nonces, intrinsic gas, and fee deduction into `PoolFeeCollector`.
  3. `EndBlock`: Distributes collected fees and block rewards proportionally across active bonded validators and their delegators (`x/staking`), completes matured unbonding and redelegation queues, tallies and executes governance proposals (`x/gov`), and computes deterministic `ValidatorUpdate` sets.
  4. `Commit`: Computes the composite SHA-256 `AppHash` across all 7 module state roots (`bank`, `staking`, `slashing`, `gov`, `transparency`, `evm`).

## 2. Finality & Reorganization Guarantee
- Instant single-block BFT finality once $>2/3$ of bonded voting power signs precommits.
- Double-signing two conflicting block hashes at the same `(height, round)` triggers an immediate 5.00% (`SlashFractionDoubleSignBps = 500`) stake slash and permanent `Tombstoned = true` removal from consensus.

