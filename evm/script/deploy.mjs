import { compileContracts } from './compile.mjs';

const RPC_URL = process.env.BETTERGOV_DEVNET_RPC_URL || 'http://127.0.0.1:8545';
const DEPLOYER = '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266';

async function rpcCall(method, params = []) {
  const res = await fetch(RPC_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  });
  const json = await res.json();
  if (json.error) {
    throw new Error(`RPC error (${method}): ${json.error.message}`);
  }
  return json.result;
}

async function main() {
  const artifacts = compileContracts();
  console.log(`[BETTERGOV Deploy] Connecting to ${RPC_URL}...`);
  const chainIdHex = await rpcCall('eth_chainId');
  console.log(`[BETTERGOV Deploy] Connected to Chain ID ${parseInt(chainIdHex, 16)} (${chainIdHex})`);

  for (const [name, artifact] of Object.entries(artifacts)) {
    const txHash = await rpcCall('eth_sendTransaction', [
      {
        from: DEPLOYER,
        to: '',
        value: '0x0',
        data: artifact.bytecode,
        gas: '0x50000',
      },
    ]);
    const receipt = await rpcCall('eth_getTransactionReceipt', [txHash]);
    console.log(`[BETTERGOV Deploy] Deployed ${name} -> ${receipt.contractAddress} (tx: ${txHash}, gasUsed: ${receipt.gasUsed})`);
  }
}

main().catch((err) => {
  console.error('[BETTERGOV Deploy] Failed:', err.message);
  process.exit(1);
});

