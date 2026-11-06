/**
 * BETTERGOV AI Security — Report Generator (Agent #9)
 * Generates reproducible Markdown & JSON security audit reports with SHA-256 provenance receipts.
 */

import crypto from 'node:crypto';
import { BaseSecurityAgent } from '../agents/index.mjs';

export class ReportGeneratorAgent extends BaseSecurityAgent {
  constructor(vault) {
    super('agent-report-generator', 'Report Generator', 'Generates reproducible security audit reports with verified provenance receipts', ['reports/export']);
    this.vault = vault;
  }

  generateAuditReport(missionMetadata = {}) {
    return this.executeSafeRead('GENERATE_AUDIT_REPORT', () => {
      const allFindings = this.vault.getAllFindings();
      const verifiedFindings = this.vault.getVerifiedFindings();
      const unverifiedFindings = allFindings.filter((f) => !f.verifyGate?.passed);

      const summary = {
        reportId: `BGOV-AUDIT-${Date.now()}`,
        generatedAt: new Date().toISOString(),
        missionName: missionMetadata.name || 'BETTERGOV Continuous Defensive Audit',
        totalFindings: allFindings.length,
        verifiedToolProvenCount: verifiedFindings.length,
        unverifiedBlockedCount: unverifiedFindings.length,
        bySeverity: {
          critical: verifiedFindings.filter((f) => f.severity === 'critical').length,
          high: verifiedFindings.filter((f) => f.severity === 'high').length,
          medium: verifiedFindings.filter((f) => f.severity === 'medium').length,
          low: verifiedFindings.filter((f) => f.severity === 'low').length,
          info: verifiedFindings.filter((f) => f.severity === 'info').length,
        },
        permissionAttestation: {
          canChangeConsensus: false,
          canAccessValidatorPrivateKeys: false,
          canExecuteArbitraryTransactions: false,
          canSeizeUserFunds: false,
        },
        findings: allFindings,
      };

      const reportSha256 = crypto
        .createHash('sha256')
        .update(JSON.stringify(summary.findings))
        .digest('hex');

      const markdown = [
        `# BETTERGOV AI Security Audit Report`,
        `- **Report ID**: \`${summary.reportId}\``,
        `- **Mission**: ${summary.missionName}`,
        `- **Generated At**: ${summary.generatedAt}`,
        `- **Findings Ledger SHA-256**: \`${reportSha256}\``,
        `- **Total Findings**: ${summary.totalFindings} (${summary.verifiedToolProvenCount} Tool-Verified, ${summary.unverifiedBlockedCount} Unverified/Gate-Blocked)`,
        ``,
        `## Verified Findings (Passed Live Verification Gate)`,
        verifiedFindings.length === 0
          ? `_No verified findings recorded._`
          : verifiedFindings
              .map(
                (f, i) =>
                  `### ${i + 1}. [${f.severity.toUpperCase()}] ${f.title}\n` +
                  `- **Target**: \`${f.targetId}\`\n` +
                  `- **Agent**: \`${f.operatorId}\`\n` +
                  `- **Provenance**: \`${f.verifyGate.provenance}\` (Verified at ${new Date(f.verifiedAt).toISOString()})\n` +
                  `- **Evidence Receipt SHA-256**: \`${f.evidence[0]?.sha256 || 'n/a'}\`\n` +
                  `- **Description**: ${f.description}\n` +
                  `- **Remediation**: ${f.remediation || 'Review protocol runbook.'}\n`
              )
              .join('\n'),
      ].join('\n');

      return {
        ...summary,
        reportSha256,
        markdown,
      };
    });
  }
}

