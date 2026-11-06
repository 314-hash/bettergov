# BETTERGOV Developer & Public Testnet Launch Guide

## 1. Local Development Network Quickstart

### Build and Test the Layer 1 Node (`bettergovd`)
```bash
cd chain
go test -v ./...
go build -o bettergovd ./cmd/bettergovd
./bettergovd init --output ../genesis/devnet/genesis.json
./bettergovd genesis-validate --file ../genesis/devnet/genesis.json
./bettergovd start --rpc-bind 127.0.0.1:8545 --admin-bind 127.0.0.1:26657
```

### Compile, Test, and Deploy Solidity Smart Contracts (`evm/`)
Includes:
- `PublicTransparencyRegistry.sol` (On-chain SHA-256 document/project/procurement anchors + PII guard)
- `TreasuryVault.sol` (Activity-gated public treasury vault + BSP Circular No. 1108 separation)
- `ValidatorSlashingEvidence.sol` (Equivocation evidence bridge)
- `NationalAssetTokenization.sol` (SEC Mitigation: Non-dividend RWA functional utility credits + activity-based rewards)
- `CivicSBT.sol` (ERC-5192 non-transferable Soulbound credentials for degrees, identities & reputation)
- `QuantumIdentity.sol` (Cryptographic agility key slots for Dilithium, Falcon, and SPHINCS+)

```bash
cd evm
node script/compile.mjs
node --test test/contracts.test.mjs
# When bettergovd is running on 127.0.0.1:8545:
node script/deploy.mjs
```

### Run the 9-Agent AI Security Suite (`ai-security/`)
```bash
cd ai-security
node --test tests/ai-security.test.mjs
node orchestration/run-audit.mjs
```

### Start the Explorer Backend & React Web App
```bash
# Terminal 1: Backend API & Explorer Service
cd backend
node src/server.mjs

# Terminal 2: Frontend Web Application
cd frontend
npm install
npm run dev
```

## 2. JSON-RPC Endpoints (`http://127.0.0.1:8545`)
- `eth_chainId` (`0x7a69` / `31337` on devnet; `0x12ccf` / `77002` on testnet)
- `net_version`
- `eth_blockNumber`
- `eth_getBalance`
- `eth_getTransactionCount`
- `eth_gasPrice`
- `eth_estimateGas`
- `eth_call`
- `eth_sendTransaction`
- `eth_getTransactionByHash`
- `eth_getTransactionReceipt`
- `eth_getBlockByNumber`
- `eth_getCode`

## 3. Public Testnet Launch Procedure
1. Validate `genesis/testnet/genesis.json` across all participating independent validator operators:
   ```bash
   ./bettergovd genesis-validate --file ../genesis/testnet/genesis.json
   ```
2. Verify all operators compute the identical `initial_app_hash`.
3. Configure sentry nodes using `validator/config/config.toml` keeping `rpc.laddr = "tcp://127.0.0.1:26657"` private.
4. Launch via `validator/scripts/join-testnet.sh`.

