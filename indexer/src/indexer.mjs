/**
 * BETTERGOV Blockchain Indexer & Snapshot Service
 * Syncs blocks, transactions, validators, governance proposals, and public transparency records.
 */

import crypto from 'node:crypto';

export class BetterGovIndexer {
  constructor(options = {}) {
    this.rpcUrl = options.rpcUrl || process.env.BETTERGOV_RPC_URL || 'http://127.0.0.1:8545';
    this.blocks = [];
    this.transactions = new Map();
    this.accounts = new Map();
    this.validators = [];
    this.proposals = [];
    this.transparencyRecords = [];
    this.contracts = [];
    this.lastSyncedHeight = 0;
  }

  seedGenesisState() {
    if (this.blocks.length > 0) return;

    const now = new Date().toISOString();
    this.validators = [
      {
        operatorAddress: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        consensusAddress: 'bgovvalcons14987616f3f21e882b90c8e7a098a3f924f501911',
        moniker: 'Sentinel-Alpha-Node',
        website: 'https://alpha.validator.bettergov.org',
        status: 'BOND_STATUS_BONDED',
        jailed: false,
        tombstoned: false,
        tokensBgov: 2500000,
        votingPower: 2500000,
        commissionBps: 500,
        uptimePercent: 99.98,
        delegatorsCount: 142,
      },
      {
        operatorAddress: '0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc',
        consensusAddress: 'bgovvalcons18723a9b0c2e41d7a9f2309b1a77e12d45a6b7822',
        moniker: 'CivicTrust-Validator',
        website: 'https://civictrust.bettergov.org',
        status: 'BOND_STATUS_BONDED',
        jailed: false,
        tombstoned: false,
        tokensBgov: 2000000,
        votingPower: 2000000,
        commissionBps: 700,
        uptimePercent: 99.95,
        delegatorsCount: 98,
      },
      {
        operatorAddress: '0x90f79bf6eb2c4f870365e785982e1f101e93b906',
        consensusAddress: 'bgovvalcons1c54911f09a8b7c6d5e4f3a2b1c0d9e8f7a6b5c33',
        moniker: 'Aegis-GovTech-Staking',
        website: 'https://aegis.bettergov.org',
        status: 'BOND_STATUS_BONDED',
        jailed: false,
        tombstoned: false,
        tokensBgov: 1800000,
        votingPower: 1800000,
        commissionBps: 600,
        uptimePercent: 99.91,
        delegatorsCount: 84,
      },
      {
        operatorAddress: '0x15d34aaf54267db7d7c367839aaf71a00a2c6a65',
        consensusAddress: 'bgovvalcons1f0123456789abcdef0123456789abcdef0123444',
        moniker: 'OpenLedger-Guard',
        website: 'https://openledger.bettergov.org',
        status: 'BOND_STATUS_BONDED',
        jailed: false,
        tombstoned: false,
        tokensBgov: 1500000,
        votingPower: 1500000,
        commissionBps: 500,
        uptimePercent: 100.0,
        delegatorsCount: 65,
      },
    ];

    this.blocks.push({
      height: 1,
      chainId: 'bettergov-devnet-1',
      blockHash: '0x' + crypto.createHash('sha256').update('bettergov-block-1').digest('hex'),
      appHash: '0xc7deac2c4cfead558c19347d39012314cfa596083bd1e2581e58b381d5b27ece',
      proposer: 'Sentinel-Alpha-Node',
      txCount: 2,
      gasUsed: 106000,
      timestamp: now,
    });
    this.lastSyncedHeight = 1;

    this.contracts = [
      {
        address: '0x22a6adc3ef254a1f71b86db589c948bab1139993',
        name: 'BGOVToken',
        compiler: 'solc-0.8.24',
        verified: true,
        deployedAtBlock: 866,
      },
      {
        address: '0xc5adfcce65c5e58821b2174f44cb1cf841cb80c0',
        name: 'PublicTransparencyRegistry',
        compiler: 'solc-0.8.24',
        verified: true,
        deployedAtBlock: 4,
      },
      {
        address: '0xa86c5d224fa577d40e52647dbc644f552cebef58',
        name: 'TreasuryVault',
        compiler: 'solc-0.8.24',
        verified: true,
        deployedAtBlock: 6,
      },
      {
        address: '0x1c44305e2efc6f489168cdd10753de0294ece831',
        name: 'ValidatorSlashingEvidence',
        compiler: 'solc-0.8.24',
        verified: true,
        deployedAtBlock: 7,
      },
      {
        address: '0x4457fdb209b93327cc0bae9be543862a34f1ab14',
        name: 'NationalAssetTokenization',
        compiler: 'solc-0.8.24',
        verified: true,
        deployedAtBlock: 3,
      },
      {
        address: '0x16d851c0135743c44fe4b9533010503b54747707',
        name: 'CivicSBT',
        compiler: 'solc-0.8.24',
        verified: true,
        deployedAtBlock: 2,
      },
      {
        address: '0x1ca6f11b5f2f880a345afb9c6f5cd048efe21e41',
        name: 'QuantumIdentity',
        compiler: 'solc-0.8.24',
        verified: true,
        deployedAtBlock: 5,
      },
    ];

    this.proposals = [
      {
        id: 1,
        title: 'BGP-1: National Open Procurement Infrastructure Grant',
        summary: 'Allocate 50,000 BGOV from Community Treasury to deploy open-source municipal procurement verifiers.',
        proposalType: 'TREASURY_SPEND',
        proposer: '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266',
        status: 'PROPOSAL_STATUS_VOTING_PERIOD',
        totalDepositBgov: 1000,
        quorumBps: 3340,
        thresholdBps: 5000,
        vetoThresholdBps: 3340,
        votes: {
          yesBgov: 4500000,
          noBgov: 150000,
          abstainBgov: 100000,
          noWithVetoBgov: 0,
        },
        votingEndTime: new Date(Date.now() + 86400 * 1000).toISOString(),
      },
    ];

    this.transparencyRecords = [
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
        timestamp: now,
      },
      {
        id: 'PROC-2026-014',
        category: 'PROCUREMENT',
        title: 'Rural Fiber Optic Backbone Phase II Tender',
        agency: 'DICT-INFRA',
        sha256Hash: '4f8b42c22dd3729b519ba6f68d2da7cc5b2d606d05daed5ad5128cc03e6c6358',
        offChainUri: 'ipfs://bafybeifiberbackbonephase2tender',
        verificationStatus: 'UNVERIFIED_HASH_ANCHOR',
        verifiedByAuditor: null,
        disclaimer:
          'ON-CHAIN RECORD PROVES CRYPTOGRAPHIC INTEGRITY AND TIMESTAMP ONLY. RECORDING A HASH ON BETTERGOV DOES NOT AUTOMATICALLY VALIDATE THE UNDERLYING DOCUMENT OR GOVERNMENT CLAIM UNLESS INDEPENDENTLY VERIFIED BY AUTHORIZED AUDITORS.',
        blockHeight: 1,
        timestamp: now,
      },
    ];
  }

  createSnapshot() {
    this.seedGenesisState();
    const stateJson = JSON.stringify({
      lastSyncedHeight: this.lastSyncedHeight,
      blocksCount: this.blocks.length,
      validatorsCount: this.validators.length,
      proposalsCount: this.proposals.length,
      transparencyCount: this.transparencyRecords.length,
    });
    return {
      snapshotId: `SNAP-H${this.lastSyncedHeight}`,
      height: this.lastSyncedHeight,
      createdAt: new Date().toISOString(),
      sha256Checksum: crypto.createHash('sha256').update(stateJson).digest('hex'),
      blocksCount: this.blocks.length,
      validatorsCount: this.validators.length,
    };
  }
}

