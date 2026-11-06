/**
 * BETTERGOV AI Security — Security Coordinator (Agent #8) & Mission Control
 * Inspired by T3MP3ST (src/mission/index.ts & src/general/index.ts)
 */

import crypto from 'node:crypto';
import {
  BaseSecurityAgent,
  ValidatorGuardianAgent,
  NetworkSentinelAgent,
  ContractAuditorAgent,
  TransactionAnalystAgent,
  GovernanceAnalystAgent,
  EvidenceCollectorAgent,
  VerificationAgent,
} from '../agents/index.mjs';
import { EvidenceVault } from '../evidence/vault.mjs';
import { ReportGeneratorAgent } from '../reports/generator.mjs';

export class TaskQueue {
  constructor() {
    this.tasks = [];
  }

  add(task) {
    this.tasks.push({
      id: task.id || crypto.randomUUID(),
      priority: Number(task.priority || 5),
      status: 'pending',
      ...task,
    });
    this.tasks.sort((a, b) => b.priority - a.priority);
  }

  getNextPending() {
    return this.tasks.find((t) => t.status === 'pending');
  }

  getAll() {
    return [...this.tasks];
  }
}

export class SecurityCoordinatorAgent extends BaseSecurityAgent {
  constructor() {
    super(
      'agent-security-coordinator',
      'Security Coordinator',
      'Coordinates multi-agent security investigations, Mission Control, and Rules of Engagement',
      ['orchestration/mission', 'orchestration/queue']
    );

    this.vault = new EvidenceVault();
    this.taskQueue = new TaskQueue();

    // Instantiate all 9 agents in the BETTERGOV AI Security Swarm
    this.validatorGuardian = new ValidatorGuardianAgent();
    this.networkSentinel = new NetworkSentinelAgent();
    this.contractAuditor = new ContractAuditorAgent();
    this.transactionAnalyst = new TransactionAnalystAgent();
    this.governanceAnalyst = new GovernanceAnalystAgent();
    this.evidenceCollector = new EvidenceCollectorAgent(this.vault);
    this.verificationAgent = new VerificationAgent(this.vault);
    this.reportGenerator = new ReportGeneratorAgent(this.vault);
  }

  getAllAgents() {
    return [
      this.validatorGuardian,
      this.networkSentinel,
      this.contractAuditor,
      this.transactionAnalyst,
      this.governanceAnalyst,
      this.evidenceCollector,
      this.verificationAgent,
      this,
      this.reportGenerator,
    ];
  }

  /**
   * Run a coordinated 9-agent defensive audit mission over live chain state & contract sources.
   */
  runFullAuditMission(input = {}) {
    return this.executeSafeRead('ORCHESTRATE_AUDIT_MISSION', () => {
      const rawFindings = [
        ...this.validatorGuardian.analyzeValidators(input.validators || [], input.signedBlocksWindow || 100),
        ...this.networkSentinel.analyzeNetwork(input.network || {}),
        ...(input.contracts || []).flatMap((c) => this.contractAuditor.auditSoliditySource(c.name, c.source)),
        ...this.transactionAnalyst.analyzeTransactions(input.transactions || [], input.treasuryThresholdBg || 100000),
        ...this.governanceAnalyst.analyzeProposals(input.proposals || []),
      ];

      for (const f of rawFindings) {
        this.evidenceCollector.collectAndStoreFinding(f);
      }

      const verifiedList = this.verificationAgent.verifyAllInVault();
      const report = this.reportGenerator.generateAuditReport({
        name: input.missionName || 'BETTERGOV Scheduled Security Sweep',
      });

      return {
        agents: this.getAllAgents().map((a) => ({
          id: a.id,
          name: a.name,
          role: a.role,
          status: a.status,
          completedTasks: a.completedTasks,
          permissions: a.permissions,
        })),
        verifiedFindingsCount: verifiedList.filter((f) => f.verifyGate?.passed).length,
        report,
      };
    });
  }
}

