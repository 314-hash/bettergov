#!/usr/bin/env bash
set -euo pipefail

echo "[BETTERGOV Validator] Building bettergovd daemon..."
cd "$(dirname "$0")/../../chain"
go build -o bettergovd ./cmd/bettergovd

echo "[BETTERGOV Validator] Validating public testnet genesis..."
./bettergovd genesis-validate --file ../genesis/testnet/genesis.json

echo "[BETTERGOV Validator] Starting validator node (Admin RPC restricted to 127.0.0.1:26657)..."
./bettergovd start --rpc-bind 127.0.0.1:8545 --admin-bind 127.0.0.1:26657

