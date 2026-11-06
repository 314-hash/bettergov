/**
 * BETTERGOV Live End-to-End On-Chain Transaction & Audit Walkthrough
 * Runs against the live bettergovd RPC (:8545) and Backend API (:4000).
 */

import crypto from 'node:crypto';

const RPC_URL = 'http://127.0.0.1:8545';
const BACKEND_URL = 'http://127.0.0.1:4000';
const SENDER = '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266';

async function rpc(method, params = []) {
  const res = await fetch(RPC_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  });
  const j = await res.json();
  if (j.error) throw new Error(j.error.message);
  return j.result;
}

async function main() {
  console.log('=== 1. Querying Live BETTERGOV Layer 1 Consensus Status ===');
  const statusRes = await fetch(`${RPC_URL}/bettergov/network/status`);
  const status = await statusRes.json();
  console.log(`Chain ID: ${status.chain_id} | EVM ID: ${status.evm_chain_id}`);
  console.log(`Latest Block Height: #${status.latest_height} | AppHash: ${status.latest_app_hash}`);
  console.log(`Active PoS Validators: ${status.active_validators} | Bonded: ${status.total_bonded} abgov\n`);

  console.log('=== 2. Executing Live On-Chain EVM Contract Call (PublicTransparencyRegistry) ===');
  const docHash = crypto.createHash('sha256').update('DPWH-2026-BRIDGE-AUDIT-REPORT').digest('hex');
  const callData = '0xa1b2c3d4' + docHash;
  const txHash = await rpc('eth_sendTransaction', [
    {
      from: SENDER,
      to: '0xc5adfcce65c5e58821b2174f44cb1cf841cb80c0',
      value: '0x0',
      data: callData,
      gas: '0x30000',
    },
  ]);
  const receipt = await rpc('eth_getTransactionReceipt', [txHash]);
  console.log(`Tx Hash: ${receipt.transactionHash}`);
  console.log(`Block #${receipt.blockNumber} | Status: ${receipt.status === 1 ? 'SUCCESS (0x1)' : 'REVERT'} | Gas Used: ${receipt.gasUsed}`);
  console.log(`Emitted Log Topic: ${receipt.logs[0]?.topics[0]}\n`);

  console.log('=== 3. Anchoring Public Transparency Document via Backend API (PII Guard Active) ===');
  const anchorRes = await fetch(`${BACKEND_URL}/api/v1/transparency/anchor`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      title: 'FY2026 Clean Energy Microgrid Generation Telemetry Batch #42',
      agency: 'DOE-RE-PH',
      sha256Hash: docHash,
      offChainUri: `ipfs://bafybei${docHash.slice(0, 28)}`,
    }),
  });
  const anchoredRecord = await anchorRes.json();
  console.log(`Anchored Record ID: ${anchoredRecord.id}`);
  console.log(`SHA-256 Digest: ${anchoredRecord.sha256Hash}`);
  console.log(`Verification Status: ${anchoredRecord.verificationStatus}\n`);

  console.log('=== 4. Running Live 9-Agent T3MP3ST-Inspired AI Security Sweep ===');
  const aiRes = await fetch(`${BACKEND_URL}/api/v1/ai-security/status`);
  const aiData = await aiRes.json();
  console.log(`Agents Active: ${aiData.agents.length}/9 | Verified Findings: ${aiData.verifiedFindingsCount}`);
  console.log(`Report ID: ${aiData.report.reportId} | Report SHA-256: ${aiData.report.reportSha256}`);
}

main().catch((err) => {
  console.error('Demo failed:', err);
  process.exit(1);
});

