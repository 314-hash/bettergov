# BETTERGOV Security, Validator Hardening & Mainnet Readiness

## 1. T3MP3ST-Inspired AI Security Subsystem (`ai-security/`)
BETTERGOV integrates a 9-agent defensive security swarm modeled on `elder-plinius/T3MP3ST`:
1. **Validator Guardian** (`ValidatorGuardianAgent`)
2. **Network Sentinel** (`NetworkSentinelAgent`)
3. **Contract Auditor** (`ContractAuditorAgent`)
4. **Transaction Analyst** (`TransactionAnalystAgent`)
5. **Governance Analyst** (`GovernanceAnalystAgent`)
6. **Evidence Collector** (`EvidenceCollectorAgent`)
7. **Verification Agent** (`VerificationAgent`)
8. **Security Coordinator** (`SecurityCoordinatorAgent`)
9. **Report Generator** (`ReportGeneratorAgent`)

### Non-Negotiable Permission & Provenance Boundaries
- Every agent executes under `createControlledPermissions` (`canChangeConsensus: false`, `canAccessValidatorPrivateKeys: false`, `canExecuteArbitraryTransactions: false`, `canSeizeUserFunds: false`).
- Every finding is gated by `gateLiveFinding()` (`ai-security/verification/gate.mjs`): findings without tool-output SHA-256 receipts cannot be marked `verified` and have their severity capped.

## 2. Validator Infrastructure Security
- **Loopback Admin RPC Enforcement**: `bettergovd` restricts `/admin/*` endpoints to `127.0.0.1` / `::1` via `ip.IsLoopback()` checks (`chain/app/rpc.go`).
- **Sentry Node Topology**: Validators in production must sit behind private WireGuard/VPN links connected only to trusted sentry nodes (`pex = false` on validator signing node, `pex = true` on public sentries).

## 3. Mainnet Readiness Checklist (`PLANNED` — Required Before Mainnet)
- [x] Native PoS Consensus, Staking, Slashing, Governance & EVM Implemented & Tested (`chain/`)
- [x] Public Transparency PII Guard & Mandatory Authenticity Disclaimers Implemented (`chain/x/transparency`, `evm/contracts/`)
- [x] T3MP3ST-Inspired 9-Agent AI Security Subsystem & Live Verification Gate Implemented (`ai-security/`)
- [ ] Reserve unique EIP-155 Chain ID in `ethereum-lists/chains` and Cosmos Chain Registry prior to Mainnet genesis
- [ ] Complete independent external third-party cryptography & smart contract security audit
- [ ] Conduct multi-operator decentralized Public Testnet dry-run ceremony

