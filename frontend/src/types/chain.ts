export type PageKey =
  | 'home'
  | 'overview'
  | 'explorer'
  | 'validators'
  | 'staking'
  | 'governance'
  | 'treasury'
  | 'records'
  | 'ai-security'
  | 'developers'
  | 'docs';

export interface ValidatorProfile {
  operatorAddress: string;
  consensusAddress: string;
  consensusPubKeyHex: string;
  moniker: string;
  website: string;
  status: 'BOND_STATUS_BONDED' | 'BOND_STATUS_UNBONDING' | 'BOND_STATUS_UNBONDED';
  jailed: boolean;
  tombstoned: boolean;
  tokensBgov: number;
  votingPowerSharePct: number;
  commissionBps: number;
  uptimePct: number;
}

export interface GovernanceProposal {
  id: number;
  title: string;
  summary: string;
  proposalType: 'TEXT' | 'PARAMETER_CHANGE' | 'TREASURY_SPEND' | 'SOFTWARE_UPGRADE';
  proposer: string;
  status:
    | 'PROPOSAL_STATUS_DEPOSIT_PERIOD'
    | 'PROPOSAL_STATUS_VOTING_PERIOD'
    | 'PROPOSAL_STATUS_PASSED'
    | 'PROPOSAL_STATUS_REJECTED'
    | 'PROPOSAL_STATUS_VETOED'
    | 'PROPOSAL_STATUS_EXECUTED';
  totalDepositBgov: number;
  yesBgov: number;
  noBgov: number;
  abstainBgov: number;
  noWithVetoBgov: number;
  quorumPct: number;
  votingEndTime: string;
}

export interface PublicTransparencyRecord {
  id: string;
  category:
    | 'PUBLIC_DOCUMENT'
    | 'GOVERNMENT_PROJECT'
    | 'PROCUREMENT'
    | 'BUDGET_ALLOCATION'
    | 'COMMUNITY_PARTICIPATION'
    | 'AUDIT_EVIDENCE'
    | 'DIGITAL_CERTIFICATE'
    | 'TIMESTAMP_RECORD';
  title: string;
  agency: string;
  sha256Hash: string;
  offChainUri: string;
  verificationStatus: 'UNVERIFIED_HASH_ANCHOR' | 'AUDITOR_VERIFIED' | 'DISPUTED' | 'REVOKED';
  verifiedByAuditor?: string;
  disclaimer: string;
  blockHeight: number;
  timestamp: string;
}

export interface AISecurityAgentInfo {
  id: string;
  name: string;
  role: string;
  status: 'idle' | 'executing';
  completedTasks: number;
  permissions: {
    canChangeConsensus: false;
    canAccessValidatorPrivateKeys: false;
    canExecuteArbitraryTransactions: false;
    canSeizeUserFunds: false;
  };
}

export interface AISecurityFinding {
  id: string;
  title: string;
  severity: 'critical' | 'high' | 'medium' | 'low' | 'info';
  targetId: string;
  operatorId: string;
  provenance: 'tool' | 'context' | 'none';
  gatePassed: boolean;
  evidenceSha256: string;
  description: string;
  remediation: string;
}

