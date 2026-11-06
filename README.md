<div align="center">
  <img src="docs/assets/logo.png" alt="BETTERGOV Logo" width="120" />
  <h1>BETTERGOV CHAIN (BGOV)</h1>
  <p><strong>Independent EVM-Compatible Proof-of-Stake Layer 1 Blockchain for Transparent Governance, Public Accountability & Decentralized Infrastructure</strong></p>
</div>

---

## 1. Overview

**BETTERGOV (`BGOV`)** is an independently operating Layer 1 blockchain built with Go (Cosmos SDK + CometBFT ABCI architecture), Solidity smart contracts, a PostgreSQL/Node indexer & Blockscout-compatible explorer, a 9-agent T3MP3ST-inspired AI security subsystem, and an 11-page React/TypeScript web application.

> 📄 **Official Whitepaper**: Read the full [BETTERGOV Chain Technical, Economic & Regulatory Whitepaper (`WHITEPAPER.md`)](WHITEPAPER.md) (also available in [`docs/WHITEPAPER.md`](docs/WHITEPAPER.md)).
> 🛡️ **Validator Operator Guide**: Follow the step-by-step [BETTERGOV Validator Operator Guide (`VALIDATOR_GUIDE.md`)](VALIDATOR_GUIDE.md) (also available in [`docs/VALIDATOR_GUIDE.md`](docs/VALIDATOR_GUIDE.md)).

### Core Capabilities
- **Native Token (`BGOV` / `abgov`)**: 18-decimal native asset with configurable genesis supply, dynamic inflation (`1.00%–8.00%`), validator/delegator rewards, and community treasury allocation (`chain/x/bank`, `chain/x/mint`).
- **Protocol-Level Proof-of-Stake (`chain/x/staking`, `chain/x/slashing`)**: Validator registration with Ed25519 consensus public keys, self-bonding, delegated staking, time-locked unbonding, transitive-redelegation locks, 5.00% double-sign slashing with permanent tombstone (`Tombstoned = true`), and 0.50% downtime slashing with temporary jail/unjail.
- **Protocol Governance (`chain/x/gov`)**: On-chain proposals (`TEXT`, `PARAMETER_CHANGE`, `TREASURY_SPEND`, `SOFTWARE_UPGRADE`), deposit period, bonded-stake voting power, quorum (`33.40%`), approval threshold (`>50.00%`), and `NoWithVeto` (`>=33.40%` burns deposit).
- **Public Transparency Infrastructure (`chain/x/transparency`, `evm/contracts/PublicTransparencyRegistry.sol`)**: 8 specialized public record types with an automated **Cleartext PII Guard** and mandatory **Authenticity Honesty Disclaimers** (`UNVERIFIED_HASH_ANCHOR` vs `AUDITOR_VERIFIED`).
- **SEC Classification Mitigation & BSP Circular No. 1108 VASP Separation**:
  - **Activity-Based Rewards (`chain/x/staking`, `TreasuryVault.sol`)**: Staking boosters (`+5%` to `+25%`) and treasury milestone releases are tied directly to active contribution proofs (generating clean energy, submitting verified crops, or voting in governance) rather than passive returns.
  - **Non-Dividend RWA Shares (`evm/contracts/NationalAssetTokenization.sol`)**: Fractional RWA units distribute output exclusively as functional utility discounts (solar energy kWh discounts or warehouse storage credits) and prohibit passive monetary dividends.
  - **Soulbound Credentials (`evm/contracts/CivicSBT.sol`)**: Degrees, civic identities, and reputations are ERC-5192 permanently locked non-transferable tokens (`SoulboundNonTransferable`) to eliminate speculative secondary markets.
  - **BSP Circular No. 1108 Separation**: Operates strictly at the crypto-utility layer with zero direct on-chain PHP fiat rails (`ValidateBSP1108CryptoUtilityOnly`).
- **Post-Quantum Cryptographic Agility (`evm/contracts/QuantumIdentity.sol`)**: Flexible key storage slots mapping algorithm types to public keys for seamless support of post-quantum signatures (**Dilithium / ML-DSA**, **Falcon-512/1024**, and **SPHINCS+ / SLH-DSA**) alongside EVM precompiles.
- **T3MP3ST-Inspired AI Security Subsystem (`ai-security/`)**: 9 specialized defensive agents (`ValidatorGuardian`, `NetworkSentinel`, `ContractAuditor`, `TransactionAnalyst`, `GovernanceAnalyst`, `EvidenceCollector`, `VerificationAgent`, `SecurityCoordinator`, `ReportGenerator`) operating with zero consensus/key/fund privileges and a provenance-strict **Live Verification Gate** (`gateLiveFinding`).

---

## 2. Implementation Status Matrix

| Component | Path | Status | Verification Command |
|---|---|---|---|
| **Layer 1 Node Daemon (`bettergovd`) & ABCI State Machine** | `chain/app`, `chain/cmd/bettergovd` | `IMPLEMENTED` | `cd chain && go test -v ./...` |
| **Native PoS Staking, Delegation, Unbonding & Redelegation** | `chain/x/staking` | `IMPLEMENTED` | `cd chain && go test -v ./tests -run TestNativeProofOfStakeLifecycle` |
| **Slashing (Double-Sign Tombstone, Downtime Window, Jail/Unjail)** | `chain/x/slashing` | `IMPLEMENTED` | `cd chain && go test -v ./tests -run TestSlashingDowntimeAndDoubleSign` |
| **On-Chain Governance (`x/gov`) & Treasury Execution** | `chain/x/gov` | `IMPLEMENTED` | `cd chain && go test -v ./tests -run TestGovernanceProposalsAndExecution` |
| **Public Transparency Module (8 Record Types + PII Guard)** | `chain/x/transparency` | `IMPLEMENTED` | `cd chain && go test -v ./tests -run TestPublicTransparencyAndPIIGuard` |
| **EVM Execution & Ethereum JSON-RPC (`eth_*`) + Private Admin RPC** | `chain/x/evm`, `chain/app/rpc.go` | `IMPLEMENTED` | `cd chain && go test -v ./tests -run TestEVMExecutionAndJSONRPCSecurity` |
| **Solidity Smart Contracts (`PublicTransparencyRegistry`, `TreasuryVault`, `ValidatorSlashingEvidence`)** | `evm/contracts` | `IMPLEMENTED` | `cd evm && npm test` |
| **T3MP3ST-Inspired 9-Agent AI Security & Live Verification Gate** | `ai-security/` | `IMPLEMENTED` | `cd ai-security && npm test` |
| **Blockchain Indexer, Explorer Service & Backend API** | `indexer/`, `explorer/`, `backend/` | `IMPLEMENTED` | `cd backend && npm test` |
| **React + TypeScript + Tailwind Web Application (11 Pages)** | `frontend/` | `IMPLEMENTED` | `cd frontend && npm test` |
| **Blockscout External Container Stack** | `explorer/config/blockscout.env` | `PARTIAL` | Configuration & Docker Compose ready; external Docker runtime optional |
| **Mainnet Launch & Third-Party Security Audit** | `genesis/mainnet/mainnet.template.json` | `PLANNED` | Pre-mainnet template only; mainnet is not live |

---

## 3. Repository Structure

```text
bettergov/
  chain/           # Go Layer 1 Blockchain (app, cmd/bettergovd, x/bank, x/mint, x/staking, x/slashing, x/gov, x/transparency, x/evm, proto, scripts, tests)
  evm/             # Solidity Smart Contracts, Hardhat & Foundry Configs, Compiler & Tests
  ai-security/     # T3MP3ST-Inspired 9-Agent Defensive Security Subsystem, Evidence Vault & Live Verification Gate
  explorer/        # Block Explorer Service & Blockscout Configuration
  frontend/        # React + TypeScript + Tailwind 11-Page Web Application
  backend/         # REST API & Explorer Gateway Server
  indexer/         # PostgreSQL Schema & Real-Time Blockchain Indexer / Snapshot Manager
  validator/       # Validator Node Configuration & Testnet Onboarding Scripts
  genesis/         # Devnet, Public Testnet, and Mainnet Template Genesis Configurations
  deployment/      # Multi-stage Dockerfile, Docker Compose & Prometheus Monitoring
  docs/            # Architecture, T3MP3ST Evaluation, Consensus, Staking, Governance, Security & Developer Guides
  .github/         # Automated GitHub Actions CI & Security Workflow
```

---

## 4. Quick Verification Commands

```bash
# 1. Verify Go Layer 1 Consensus, Staking, Slashing, Governance, Transparency & EVM
cd chain
go test -v ./...
go build -o bettergovd ./cmd/bettergovd
./bettergovd genesis-validate --file ../genesis/devnet/genesis.json
./bettergovd genesis-validate --file ../genesis/testnet/genesis.json

# 2. Verify EVM Solidity Smart Contracts
cd ../evm
npm test

# 3. Verify 9-Agent AI Security Swarm & Live Verification Gate
cd ../ai-security
npm test
npm run audit

# 4. Verify Backend API, Indexer & Explorer Gateway
cd ../backend
npm test

# 5. Verify Frontend 11-Page Web Application
cd ../frontend
npm test
```

---

## 5. License

MIT License. See [LICENSE](LICENSE).

