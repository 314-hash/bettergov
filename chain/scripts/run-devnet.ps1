# Start local BETTERGOV Layer 1 devnet node
$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\.."
Write-Host "[BETTERGOV] Building bettergovd..."
go build -o bettergovd.exe ./cmd/bettergovd
./bettergovd.exe init --output ../genesis/devnet/genesis.json
./bettergovd.exe genesis-validate --file ../genesis/devnet/genesis.json
./bettergovd.exe start --rpc-bind 127.0.0.1:8545 --admin-bind 127.0.0.1:26657

