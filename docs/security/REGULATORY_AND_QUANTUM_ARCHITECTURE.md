# BETTERGOV Regulatory Perimeter Mitigation & Post-Quantum Cryptographic Agility

## 1. SEC Classification Mitigation Architecture

To ensure BETTERGOV operates as a functional public-utility infrastructure protocol rather than an unregistered investment contract or security, three protocol-enforced mechanisms are built directly into the Go Layer 1 (`chain/x/staking`) and EVM smart contracts (`evm/contracts/`):

### 1.1 Activity-Based Rewards (`x/staking` & `NationalAssetTokenization.sol` & `TreasuryVault.sol`)
- **No Passive Investment Returns**: Staking reward boosters (`ActivityBoostBps`, up to `+2500 bps` / `+25%`) and treasury milestone releases (`TreasuryVault.releaseMilestone`) are gated by verifiable **Active Contributions**:
  - `CLEAN_ENERGY_GENERATION`: Verified kWh generation telemetry anchored by authorized meter verifiers.
  - `CROP_HARVEST_SUBMISSION`: Verified agricultural batch and warehouse intake proofs.
  - `GOVERNANCE_PROPOSAL_VOTE`: Active participation in protocol governance proposals.
  - `PUBLIC_AUDIT_ATTESTATION`: Independent verification of public transparency records.
- In `NationalAssetTokenization.sol`, calling `claimFunctionalUtilityCredits` with `totalActivityPoints == 0` reverts immediately with `PassiveReturnProhibitedRequiresActiveContribution()`.

### 1.2 Non-Dividend RWA Shares (`NationalAssetTokenization.sol`)
- Fractional Real-World Asset (RWA) units represent functional access to physical civic infrastructure (solar grids, agricultural storage depots, irrigation systems, cold-chain facilities).
- **Functional Utility Discounts Only**: Output is issued exclusively as non-monetary **Functional Utility Credits** (`UtilityCreditType`: `SOLAR_ENERGY_KWH_DISCOUNT`, `WAREHOUSE_STORAGE_CREDIT`, `IRRIGATION_WATER_ALLOCATION`, `COLD_CHAIN_LOGISTICS_OFFSET`) redeemable for service usage via `redeemFunctionalUtilityCredits`.
- **Monetary Dividends Prohibited**: `NationalAssetTokenization.sol` explicitly rejects native currency value deposits (`receive() external payable { revert MonetaryDividendsAndFiatRailsProhibited(); }`) and contains zero dividend distribution functions.

### 1.3 Soulbound Token (SBT) Credentials (`CivicSBT.sol`)
- Academic degrees (`ACADEMIC_DEGREE_LEVEL`), civic identity tiers (`CIVIC_IDENTITY_TIER`), public auditor reputations (`PUBLIC_AUDITOR_REPUTATION`), and producer badges are issued as ERC-5192 permanently locked **Soulbound Tokens** (`locked(tokenId) == true`).
- Calling `transferFrom`, `safeTransferFrom`, `approve`, or `setApprovalForAll` unconditionally reverts with `SoulboundNonTransferable()`, eliminating speculative secondary markets for identity and reputation credentials.

---

## 2. BSP Circular No. 1108 (VASP Separation)

- **Strict Crypto-Utility Layer Execution**: BETTERGOV operates exclusively on native `BGOV` (`abgov`) utility units and functional utility credits.
- **Zero On-Chain Fiat (PHP) Rails**: Neither the Go Layer 1 (`bank.ValidateBSP1108CryptoUtilityOnly`) nor any Solidity smart contract (`PublicTransparencyRegistry.sol`, `TreasuryVault.sol`, `NationalAssetTokenization.sol`, `CivicSBT.sol`, `QuantumIdentity.sol`) implements direct Philippine Peso (`PHP`) fiat custody, fiat minting, or remittance settlement rails.
- **Off-Chain Licensed VASP Boundary**: Any conversion between fiat currency (`PHP`) and utility tokens occurs strictly off-chain through independently licensed Virtual Asset Service Providers (VASPs), keeping on-chain smart contracts outside unlicensed remittance or e-money issuer classifications.
- **Automated AI Security Enforcement**: `ContractAuditorAgent` (`ai-security/agents/index.mjs`) scans smart contract source code for prohibited on-chain fiat remittance or passive dividend functions and flags violations in the Evidence Vault.

---

## 3. Cryptographic Agility (`QuantumIdentity.sol`)

To protect long-lived public records, validator identities, and citizen commitments against future quantum adversaries (CRQCs), `evm/contracts/QuantumIdentity.sol` implements algorithm-agile key storage slots:

| `AlgorithmType` Enum | Scheme Category | Key Standard / Purpose |
|---|---|---|
| `ECDSA_SECP256K1` | Classical Elliptic Curve | Standard Ethereum 65-byte secp256k1 verification |
| `ED25519` | Classical Edwards Curve | Standard CometBFT 32-byte consensus validator key |
| `DILITHIUM_ML_DSA_65` | Post-Quantum Lattice | NIST FIPS 204 (CRYSTALS-Dilithium) primary digital signature |
| `FALCON_512` | Post-Quantum NTRU Lattice | Compact signature size for bandwidth-constrained transactions |
| `FALCON_1024` | Post-Quantum NTRU Lattice | High-security NIST Level 5 lattice signature |
| `SPHINCS_PLUS_SHA256` | Post-Quantum Stateless Hash | NIST FIPS 205 (SLH-DSA) conservative hash-based fallback |

- **Flexible Key Slots**: Maps `subjectAddress => AlgorithmType => AgileKeySlot` (`bytes publicKey`, `bytes32 keyCommitmentHash`, `uint64 keyEpoch`, `bool active`, `bool revoked`).
- **Seamless EVM Precompile Routing**: `setAlgorithmPrecompileVerifier(AlgorithmType, address)` binds each post-quantum algorithm to its EVM precompile address as soon as activated via network upgrade, enabling `verifyWithAgilePrecompile()` without migrating user account addresses.

