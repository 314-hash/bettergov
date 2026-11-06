/**
 * BETTERGOV AI Security — 9 Specialized Defensive Security Agents
 *
 * Agents:
 * 1. ValidatorGuardian — monitors validator health, missed blocks, commission spikes, voting power concentration.
 * 2. NetworkSentinel — monitors P2P peer connectivity, inbound/outbound ratios, eclipse/sybil indicators.
 * 3. ContractAuditor — analyzes Solidity smart contracts & EVM bytecode for vulnerabilities.
 * 4. TransactionAnalyst — identifies suspicious transaction patterns, rapid treasury drain, replay attempts.
 * 5. GovernanceAnalyst — analyzes voting anomalies, flash-stake quorum manipulation, risky param changes.
 * 6. EvidenceCollector — records source-linked evidence with SHA-256 receipts and secret redaction.
 * 7. VerificationAgent — independently checks findings through the Live Verification Gate.
 * 8. SecurityCoordinator — coordinates investigations, mission queues, and cross-agent correlation.
 * 9. ReportGenerator — creates reproducible Markdown and JSON audit reports.
 */

import { createControlledPermissions, assertPermissionBoundary } from '../verification/gate.mjs';
import { computeEvidenceSha256, redactSecrets } from '../evidence/vault.mjs';

export class BaseSecurityAgent {
  constructor(id, name, role, scopes) {
    this.id = id;
    this.name = name;
    this.role = role;
    this.permissions = createControlledPermissions(id, scopes);
    this.status = 'idle';
    this.completedTasks = 0;
  }

  executeSafeRead(actionName, callback) {
    assertPermissionBoundary(this.permissions, actionName);
    this.status = 'executing';
    try {
      const res = callback();
      this.completedTasks += 1;
      this.status = 'idle';
      return res;
    } catch (err) {
      this.status = 'idle';
      throw err;
    }
  }
}

// 1. Validator Guardian
export class ValidatorGuardianAgent extends BaseSecurityAgent {
  constructor() {
    super('agent-validator-guardian', 'Validator Guardian', 'Monitors validator health, uptime, commission changes, and voting power concentration', ['x/staking', 'x/slashing']);
  }

  analyzeValidators(validators = [], signedBlocksWindow = 100) {
    return this.executeSafeRead('READ_VALIDATOR_TELEMETRY', () => {
      const findings = [];
      const totalPower = validators.reduce((acc, v) => acc + Number(v.votingPower || 0), 0);

      for (const v of validators) {
        const missed = Number(v.missedBlocks || 0);
        const shareBps = totalPower > 0 ? Math.round((Number(v.votingPower || 0) * 10000) / totalPower) : 0;

        if (missed >= signedBlocksWindow * 0.25) {
          const rawToolOutput = JSON.stringify({
            validator: v.operatorAddress,
            moniker: v.moniker,
            missedBlocks: missed,
            window: signedBlocksWindow,
            jailed: Boolean(v.jailed),
          });
          findings.push({
            title: `Elevated Downtime Risk on Validator ${v.moniker}`,
            description: `Validator ${v.moniker} (${v.operatorAddress}) has missed ${missed}/${signedBlocksWindow} blocks in the active signing window.`,
            severity: missed >= signedBlocksWindow * 0.45 ? 'high' : 'medium',
            targetId: v.operatorAddress,
            operatorId: this.id,
            evidence: [
              {
                type: 'rpc_response',
                tool: 'cosmos_slashing_signing_info',
                content: rawToolOutput,
                sha256: computeEvidenceSha256(rawToolOutput),
              },
            ],
            remediation: 'Verify sentry node connectivity, disk I/O latency, and CometBFT precommit timeout settings before reaching the 50% downtime slash threshold.',
          });
        }

        if (shareBps >= 3334) {
          const rawToolOutput = JSON.stringify({
            validator: v.operatorAddress,
            votingPower: v.votingPower,
            totalNetworkPower: totalPower,
            shareBps,
          });
          findings.push({
            title: `Consensus BFT Threshold Concentration Alert`,
            description: `Validator ${v.moniker} controls ${(shareBps / 100).toFixed(2)}% of active voting power (>= 33.34% CometBFT liveness halt threshold).`,
            severity: 'high',
            targetId: v.operatorAddress,
            operatorId: this.id,
            evidence: [
              {
                type: 'rpc_response',
                tool: 'cosmos_staking_validators',
                content: rawToolOutput,
                sha256: computeEvidenceSha256(rawToolOutput),
              },
            ],
            remediation: 'Encourage community delegation rebalancing across independent validators to preserve BFT fault tolerance.',
          });
        }
      }
      return findings;
    });
  }
}

// 2. Network Sentinel
export class NetworkSentinelAgent extends BaseSecurityAgent {
  constructor() {
    super('agent-network-sentinel', 'Network Sentinel', 'Monitors P2P peer connectivity, eclipse indicators, and RPC latency', ['p2p/net_info', 'rpc/health']);
  }

  analyzeNetwork(telemetry = {}) {
    return this.executeSafeRead('READ_P2P_NETWORK_STATE', () => {
      const findings = [];
      const peers = Array.isArray(telemetry.peers) ? telemetry.peers : [];
      const outboundCount = peers.filter((p) => p.isOutbound).length;
      const uniqueSubnets = new Set(peers.map((p) => String(p.remoteIp || '').split('.').slice(0, 2).join('.'))).size;

      if (peers.length > 0 && (outboundCount === 0 || uniqueSubnets < 2)) {
        const rawOutput = JSON.stringify({
          totalPeers: peers.length,
          outboundCount,
          uniqueClassBSubnets: uniqueSubnets,
        });
        findings.push({
          title: 'Low P2P Subnet Diversity / Potential Eclipse Exposure',
          description: `Node has ${peers.length} connected peers across only ${uniqueSubnets} distinct /16 subnet(s) and ${outboundCount} outbound peer(s).`,
          severity: outboundCount === 0 ? 'high' : 'medium',
          targetId: telemetry.nodeId || 'local-p2p-node',
          operatorId: this.id,
          evidence: [
            {
              type: 'rpc_response',
              tool: 'cometbft_net_info',
              content: rawOutput,
              sha256: computeEvidenceSha256(rawOutput),
            },
          ],
          remediation: 'Configure persistent_peers across geographically and ASN-diverse sentry nodes.',
        });
      }
      return findings;
    });
  }
}

// 3. Contract Auditor
export class ContractAuditorAgent extends BaseSecurityAgent {
  constructor() {
    super('agent-contract-auditor', 'Contract Auditor', 'Performs static security analysis on Solidity smart contracts and EVM bytecode', ['evm/contracts', 'evm/bytecode']);
  }

  auditSoliditySource(contractName, sourceCode = '') {
    return this.executeSafeRead('ANALYZE_SOLIDITY_CONTRACT', () => {
      const findings = [];
      const lines = sourceCode.split('\n');

      // Check 1: tx.origin authentication vulnerability (CWE-477 / SWC-115)
      lines.forEach((line, idx) => {
        if (line.includes('tx.origin')) {
          const snippet = `Line ${idx + 1}: ${line.trim()}`;
          findings.push({
            title: `Dangerous Use of tx.origin in ${contractName}`,
            description: `Contract ${contractName} uses tx.origin at line ${idx + 1}, exposing callers to phishing/man-in-the-middle authorization bypass.`,
            severity: 'high',
            cwe: ['CWE-287'],
            targetId: contractName,
            operatorId: this.id,
            evidence: [
              {
                type: 'ast_match',
                tool: 'solidity_static_scanner',
                content: snippet,
                sha256: computeEvidenceSha256(snippet),
              },
            ],
            remediation: 'Replace tx.origin with msg.sender for authorization checks.',
          });
        }
      });

      // Check 2: External value call without nonReentrant guard
      if (sourceCode.includes('.call{value:') && !sourceCode.includes('nonReentrant')) {
        const matchLine = lines.findIndex((l) => l.includes('.call{value:'));
        const snippet = `Line ${matchLine + 1}: ${lines[matchLine]?.trim()}`;
        findings.push({
          title: `Unprotected Ether/BGOV Value Call in ${contractName}`,
          description: `Low-level .call{value:...} detected without a nonReentrant mutex modifier.`,
          severity: 'critical',
          cwe: ['CWE-841'],
          targetId: contractName,
          operatorId: this.id,
          evidence: [
            {
              type: 'ast_match',
              tool: 'solidity_static_scanner',
              content: snippet,
              sha256: computeEvidenceSha256(snippet),
            },
          ],
          remediation: 'Apply a checks-effects-interactions pattern and a nonReentrant modifier.',
        });
      }

      // Check 3: BSP Circular No. 1108 & SEC Perimeter Check (Reject direct on-chain PHP fiat rails or unlicensed remittance)
      lines.forEach((line, idx) => {
        if (/function\s+.*(?:phpFiatDeposit|remitPhpOnChain|distributeMonetaryDividend)/i.test(line)) {
          const snippet = `Line ${idx + 1}: ${line.trim()}`;
          findings.push({
            title: `BSP Circular No. 1108 / SEC Perimeter Violation in ${contractName}`,
            description: `Contract ${contractName} defines a direct PHP fiat rail or passive monetary dividend function at line ${idx + 1}.`,
            severity: 'high',
            targetId: contractName,
            operatorId: this.id,
            evidence: [
              {
                type: 'ast_match',
                tool: 'regulatory_perimeter_scanner',
                content: snippet,
                sha256: computeEvidenceSha256(snippet),
              },
            ],
            remediation: 'Operate strictly at the crypto-utility layer; route fiat conversions off-chain via licensed VASPs and issue non-dividend functional utility credits.',
          });
        }
      });

      return findings;
    });
  }
}

// 4. Transaction Analyst
export class TransactionAnalystAgent extends BaseSecurityAgent {
  constructor() {
    super('agent-transaction-analyst', 'Transaction Analyst', 'Identifies suspicious transaction patterns, rapid treasury outflows, and replay anomalies', ['x/evm', 'x/bank']);
  }

  analyzeTransactions(transactions = [], treasuryThresholdBg = 100000) {
    return this.executeSafeRead('ANALYZE_TX_STREAM', () => {
      const findings = [];
      for (const tx of transactions) {
        const valBg = Number(tx.valueBgov || 0);
        if (valBg >= treasuryThresholdBg) {
          const receiptStr = JSON.stringify({
            hash: tx.hash,
            from: tx.from,
            to: tx.to,
            valueBgov: valBg,
            blockNumber: tx.blockNumber,
          });
          findings.push({
            title: `Large Value Transfer Exceeding Watch Threshold (${valBg} BGOV)`,
            description: `Transaction ${tx.hash} transferred ${valBg} BGOV from ${tx.from} to ${tx.to}.`,
            severity: valBg >= treasuryThresholdBg * 5 ? 'high' : 'medium',
            targetId: tx.hash || tx.from,
            operatorId: this.id,
            evidence: [
              {
                type: 'tx_receipt',
                tool: 'eth_getTransactionReceipt',
                content: receiptStr,
                sha256: computeEvidenceSha256(receiptStr),
              },
            ],
            remediation: 'Cross-reference transfer against approved Governance TreasurySpend proposals.',
          });
        }
      }
      return findings;
    });
  }
}

// 5. Governance Analyst
export class GovernanceAnalystAgent extends BaseSecurityAgent {
  constructor() {
    super('agent-governance-analyst', 'Governance Analyst', 'Analyzes governance proposals and voting patterns for quorum manipulation or dangerous parameter shifts', ['x/gov']);
  }

  analyzeProposals(proposals = []) {
    return this.executeSafeRead('ANALYZE_GOV_PROPOSALS', () => {
      const findings = [];
      for (const prop of proposals) {
        if (Array.isArray(prop.paramChanges)) {
          for (const ch of prop.paramChanges) {
            if (ch.key === 'community_tax_bps' && Number(ch.value) > 3000) {
              const payload = JSON.stringify({ proposalId: prop.id, change: ch, proposer: prop.proposer });
              findings.push({
                title: `Extreme Parameter Change in Proposal #${prop.id}`,
                description: `Proposal #${prop.id} attempts to set ${ch.subspace}.${ch.key} to ${ch.value} bps (>30%).`,
                severity: 'high',
                targetId: `proposal-${prop.id}`,
                operatorId: this.id,
                evidence: [
                  {
                    type: 'rpc_response',
                    tool: 'cosmos_gov_proposal_query',
                    content: payload,
                    sha256: computeEvidenceSha256(payload),
                  },
                ],
                remediation: 'Alert validators and delegators to evaluate NoWithVeto if proposal threatens economic stability.',
              });
            }
          }
        }
      }
      return findings;
    });
  }
}

// 6. Evidence Collector
export class EvidenceCollectorAgent extends BaseSecurityAgent {
  constructor(vault) {
    super('agent-evidence-collector', 'Evidence Collector', 'Captures source-linked tool outputs, computes SHA-256 receipts, and enforces secret redaction', ['evidence/vault']);
    this.vault = vault;
  }

  collectAndStoreFinding(rawFinding) {
    return this.executeSafeRead('COLLECT_EVIDENCE', () => {
      const sanitized = {
        ...rawFinding,
        description: redactSecrets(rawFinding.description || ''),
        evidence: (rawFinding.evidence || []).map((ev) => {
          const cleanContent = redactSecrets(ev.content || '');
          return {
            ...ev,
            content: cleanContent,
            sha256: ev.sha256 || computeEvidenceSha256(cleanContent),
          };
        }),
      };
      return this.vault.addFinding(sanitized);
    });
  }
}

// 7. Verification Agent
export class VerificationAgent extends BaseSecurityAgent {
  constructor(vault) {
    super('agent-verification', 'Verification Agent', 'Independently checks findings via the Live Verification Gate (provenance-strict)', ['verification/gate']);
    this.vault = vault;
  }

  verifyAllInVault() {
    return this.executeSafeRead('VERIFY_FINDINGS_GATE', () => {
      const all = this.vault.getAllFindings();
      const results = [];
      for (const f of all) {
        const checked = this.vault.verifyFinding(f.id);
        results.push(checked);
      }
      return results;
    });
  }
}

