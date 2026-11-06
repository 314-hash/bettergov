import type {
  ValidatorProfile,
  GovernanceProposal,
  PublicTransparencyRecord,
  AISecurityAgentInfo,
  AISecurityFinding,
} from '../types/chain';

export const BETTERGOV_DEVNET_CHAIN = {
  chainIdHex: '0x7a69', // 31337
  chainIdDec: 31337,
  cosmosChainId: 'bettergov-devnet-1',
  chainName: 'BETTERGOV Layer 1 Devnet',
  nativeCurrency: {
    name: 'BetterGov',
    symbol: 'BGOV',
    decimals: 18,
  },
  rpcUrls: ['http://127.0.0.1:8545'],
  blockExplorerUrls: ['http://127.0.0.1:5173'],
};

export const INITIAL_VALIDATORS: ValidatorProfile[] = [
  {
    operatorAddress: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
    consensusAddress: 'bgovvalcons14987616f3f21e882b90c8e7a098a3f924f501911',
    consensusPubKeyHex: '1111111111111111111111111111111111111111111111111111111111111111',
    moniker: 'Sentinel-Alpha-Node',
    website: 'https://alpha.validator.bettergov.org',
    status: 'BOND_STATUS_BONDED',
    jailed: false,
    tombstoned: false,
    tokensBgov: 2500000,
    votingPowerSharePct: 32.05,
    commissionBps: 500,
    uptimePct: 99.98,
  },
  {
    operatorAddress: '0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc',
    consensusAddress: 'bgovvalcons18723a9b0c2e41d7a9f2309b1a77e12d45a6b7822',
    consensusPubKeyHex: '2222222222222222222222222222222222222222222222222222222222222222',
    moniker: 'CivicTrust-Validator',
    website: 'https://civictrust.bettergov.org',
    status: 'BOND_STATUS_BONDED',
    jailed: false,
    tombstoned: false,
    tokensBgov: 2000000,
    votingPowerSharePct: 25.64,
    commissionBps: 700,
    uptimePct: 99.95,
  },
  {
    operatorAddress: '0x90f79bf6eb2c4f870365e785982e1f101e93b906',
    consensusAddress: 'bgovvalcons1c54911f09a8b7c6d5e4f3a2b1c0d9e8f7a6b5c33',
    consensusPubKeyHex: '3333333333333333333333333333333333333333333333333333333333333333',
    moniker: 'Aegis-GovTech-Staking',
    website: 'https://aegis.bettergov.org',
    status: 'BOND_STATUS_BONDED',
    jailed: false,
    tombstoned: false,
    tokensBgov: 1800000,
    votingPowerSharePct: 23.08,
    commissionBps: 600,
    uptimePct: 99.91,
  },
  {
    operatorAddress: '0x15d34aaf54267db7d7c367839aaf71a00a2c6a65',
    consensusAddress: 'bgovvalcons1f0123456789abcdef0123456789abcdef0123444',
    consensusPubKeyHex: '4444444444444444444444444444444444444444444444444444444444444444',
    moniker: 'OpenLedger-Guard',
    website: 'https://openledger.bettergov.org',
    status: 'BOND_STATUS_BONDED',
    jailed: false,
    tombstoned: false,
    tokensBgov: 1500000,
    votingPowerSharePct: 19.23,
    commissionBps: 500,
    uptimePct: 100.0,
  },
];

export const INITIAL_PROPOSALS: GovernanceProposal[] = [
  {
    id: 1,
    title: 'BGP-1: National Open Procurement Infrastructure Grant',
    summary:
      'Allocate 50,000 BGOV from the Community Treasury to deploy open-source municipal procurement hash verifiers and COA audit relays.',
    proposalType: 'TREASURY_SPEND',
    proposer: '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266',
    status: 'PROPOSAL_STATUS_VOTING_PERIOD',
    totalDepositBgov: 1000,
    yesBgov: 4500000,
    noBgov: 150000,
    abstainBgov: 100000,
    noWithVetoBgov: 0,
    quorumPct: 60.9,
    votingEndTime: '2026-11-08T00:00:00Z',
  },
  {
    id: 2,
    title: 'BGP-2: Slashing Window Tightening for High-Assurance Finality',
    summary:
      'Adjust x/slashing min_signed_per_window_bps from 5000 (50%) to 6000 (60%) over a 100-block signing window.',
    proposalType: 'PARAMETER_CHANGE',
    proposer: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
    status: 'PROPOSAL_STATUS_EXECUTED',
    totalDepositBgov: 1500,
    yesBgov: 6300000,
    noBgov: 0,
    abstainBgov: 0,
    noWithVetoBgov: 0,
    quorumPct: 80.77,
    votingEndTime: '2026-11-01T00:00:00Z',
  },
];

export const INITIAL_RECORDS: PublicTransparencyRecord[] = [
  {
    id: 'DOC-2026-001',
    category: 'PUBLIC_DOCUMENT',
    title: 'FY2026 National Infrastructure Audit Summary',
    agency: 'COA-PH',
    sha256Hash: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
    offChainUri: 'ipfs://bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi',
    verificationStatus: 'AUDITOR_VERIFIED',
    verifiedByAuditor: '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266',
    disclaimer:
      'ON-CHAIN RECORD PROVES CRYPTOGRAPHIC INTEGRITY AND TIMESTAMP ONLY. RECORDING A HASH ON BETTERGOV DOES NOT AUTOMATICALLY VALIDATE THE UNDERLYING DOCUMENT OR GOVERNMENT CLAIM UNLESS INDEPENDENTLY VERIFIED BY AUTHORIZED AUDITORS.',
    blockHeight: 1,
    timestamp: '2026-11-05T16:00:00Z',
  },
  {
    id: 'PROC-2026-014',
    category: 'PROCUREMENT',
    title: 'Rural Fiber Optic Backbone Phase II Public Tender',
    agency: 'DICT-INFRA',
    sha256Hash: '4f8b42c22dd3729b519ba6f68d2da7cc5b2d606d05daed5ad5128cc03e6c6358',
    offChainUri: 'ipfs://bafybeifiberbackbonephase2tender',
    verificationStatus: 'UNVERIFIED_HASH_ANCHOR',
    disclaimer:
      'ON-CHAIN RECORD PROVES CRYPTOGRAPHIC INTEGRITY AND TIMESTAMP ONLY. RECORDING A HASH ON BETTERGOV DOES NOT AUTOMATICALLY VALIDATE THE UNDERLYING DOCUMENT OR GOVERNMENT CLAIM UNLESS INDEPENDENTLY VERIFIED BY AUTHORIZED AUDITORS.',
    blockHeight: 1,
    timestamp: '2026-11-05T16:05:00Z',
  },
  {
    id: 'PROJ-2026-009',
    category: 'GOVERNMENT_PROJECT',
    title: 'Metro Flood Control Telemetry & Pumping Modernization',
    agency: 'DPWH-NCR',
    sha256Hash: '9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08',
    offChainUri: 'ipfs://bafybeifloodcontroltelemetry2026',
    verificationStatus: 'AUDITOR_VERIFIED',
    verifiedByAuditor: '0x15d34aaf54267db7d7c367839aaf71a00a2c6a65',
    disclaimer:
      'ON-CHAIN RECORD PROVES CRYPTOGRAPHIC INTEGRITY AND TIMESTAMP ONLY. RECORDING A HASH ON BETTERGOV DOES NOT AUTOMATICALLY VALIDATE THE UNDERLYING DOCUMENT OR GOVERNMENT CLAIM UNLESS INDEPENDENTLY VERIFIED BY AUTHORIZED AUDITORS.',
    blockHeight: 1,
    timestamp: '2026-11-05T16:10:00Z',
  },
];

export const AI_SECURITY_AGENTS: AISecurityAgentInfo[] = [
  {
    id: 'agent-validator-guardian',
    name: '1. Validator Guardian',
    role: 'Monitors validator health, missed-block windows, commission spikes & voting power concentration',
    status: 'idle',
    completedTasks: 18,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
  {
    id: 'agent-network-sentinel',
    name: '2. Network Sentinel',
    role: 'Monitors P2P peer connectivity, ASN/subnet diversity, and eclipse/sybil indicators',
    status: 'idle',
    completedTasks: 24,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
  {
    id: 'agent-contract-auditor',
    name: '3. Contract Auditor',
    role: 'Static AST & EVM bytecode analysis for reentrancy, tx.origin, and access-control flaws',
    status: 'idle',
    completedTasks: 12,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
  {
    id: 'agent-transaction-analyst',
    name: '4. Transaction Analyst',
    role: 'Identifies abnormal treasury outflows, wash trading, and nonce replay attempts',
    status: 'idle',
    completedTasks: 31,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
  {
    id: 'agent-governance-analyst',
    name: '5. Governance Analyst',
    role: 'Detects flash-stake quorum manipulation, last-block whale swings, and risky param changes',
    status: 'idle',
    completedTasks: 9,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
  {
    id: 'agent-evidence-collector',
    name: '6. Evidence Collector',
    role: 'Records source-linked tool outputs with SHA-256 receipts & enforces secret redaction',
    status: 'idle',
    completedTasks: 42,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
  {
    id: 'agent-verification',
    name: '7. Verification Agent',
    role: 'Runs T3MP3ST-inspired Live Verification Gate (enforces provenance: tool before verify)',
    status: 'idle',
    completedTasks: 42,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
  {
    id: 'agent-security-coordinator',
    name: '8. Security Coordinator',
    role: 'Op General & Mission Control task scheduler with defensive Rules of Engagement',
    status: 'idle',
    completedTasks: 15,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
  {
    id: 'agent-report-generator',
    name: '9. Report Generator',
    role: 'Compiles reproducible Markdown & JSON security audit reports with provenance hashes',
    status: 'idle',
    completedTasks: 15,
    permissions: {
      canChangeConsensus: false,
      canAccessValidatorPrivateKeys: false,
      canExecuteArbitraryTransactions: false,
      canSeizeUserFunds: false,
    },
  },
];

export const INITIAL_AI_FINDINGS: AISecurityFinding[] = [
  {
    id: 'FND-2026-01',
    title: 'Production Contracts Verified Free of Reentrancy & tx.origin Flaws',
    severity: 'info',
    targetId: 'evm/contracts/*.sol',
    operatorId: 'agent-contract-auditor',
    provenance: 'tool',
    gatePassed: true,
    evidenceSha256: 'a87ff679a2f3e71d9181a67b7542122c851f23b162789a01f98a22341b651109',
    description:
      'Static scan of PublicTransparencyRegistry.sol, TreasuryVault.sol, and ValidatorSlashingEvidence.sol confirmed nonReentrant and onlyGovernance modifiers on all value transfers.',
    remediation: 'Maintain automated CI static analysis on every pull request.',
  },
  {
    id: 'FND-2026-02',
    title: 'Unbacked Model Hypothesis Blocked by Live Verification Gate (Demo)',
    severity: 'low',
    targetId: 'validator-set',
    operatorId: 'agent-verification',
    provenance: 'none',
    gatePassed: false,
    evidenceSha256: '0000000000000000000000000000000000000000000000000000000000000000',
    description:
      'Demonstrates T3MP3ST gateLiveFinding: a model-asserted claim without tool-output evidence is automatically blocked from verification and capped to low severity.',
    remediation: 'Attach deterministic RPC or AST tool output receipt before re-running verifyFinding().',
  },
];

export async function computeBrowserSha256(text: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(text);
  const digest = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

export async function callJsonRpc(rpcUrl: string, method: string, params: unknown[] = []): Promise<unknown> {
  const response = await fetch(rpcUrl, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: Date.now(),
      method,
      params,
    }),
  });
  const json = await response.json();
  if (json.error) {
    throw new Error(json.error.message || 'JSON-RPC error');
  }
  return json.result;
}

