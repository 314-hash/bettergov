/**
 * BETTERGOV AI Security — Live Verification Gate & Permission Boundary Guard
 * Inspired by T3MP3ST (src/evidence/gate.ts & docs/VERIFIED_PROVENANCE.md)
 *
 * Enforces two non-negotiable invariants:
 * 1. Provenance-Strict Honesty Gate: A finding is ONLY marked verified when backed by
 *    real machine/tool output ('output', 'rpc_response', 'tx_receipt', 'ast_match', 'block_header', 'log').
 *    Model prose or unbacked assertions cannot pass the verification gate, and their severity is capped.
 * 2. Immutable Agent Permission Boundary: No agent can modify consensus, access validator private keys,
 *    execute arbitrary transactions, or seize user funds.
 */

export const TOOL_EVIDENCE_TYPES = new Set([
  'output',
  'command',
  'response',
  'request',
  'log',
  'file',
  'rpc_response',
  'tx_receipt',
  'ast_match',
  'block_header',
]);

export const FORBIDDEN_CAPABILITIES = Object.freeze([
  'CHANGE_CONSENSUS',
  'ACCESS_VALIDATOR_PRIVATE_KEYS',
  'EXECUTE_ARBITRARY_TRANSACTIONS',
  'SEIZE_USER_FUNDS',
]);

/**
 * Create a frozen, read-only permission profile for a BETTERGOV AI Security Agent.
 */
export function createControlledPermissions(agentId, allowedReadScopes = []) {
  return Object.freeze({
    agentId,
    readOnlyTelemetry: true,
    allowedReadScopes: Object.freeze([...allowedReadScopes]),
    canChangeConsensus: false,
    canAccessValidatorPrivateKeys: false,
    canExecuteArbitraryTransactions: false,
    canSeizeUserFunds: false,
  });
}

/**
 * Enforce that an agent action does not violate protocol safety boundaries.
 */
export function assertPermissionBoundary(permissions, requestedAction) {
  if (!permissions || permissions.canChangeConsensus !== false ||
      permissions.canAccessValidatorPrivateKeys !== false ||
      permissions.canExecuteArbitraryTransactions !== false ||
      permissions.canSeizeUserFunds !== false) {
    throw new Error('SECURITY_VIOLATION: Agent permission object attempts to enable a forbidden consensus/key/fund capability');
  }
  const upper = String(requestedAction || '').toUpperCase();
  for (const forbidden of FORBIDDEN_CAPABILITIES) {
    if (upper.includes(forbidden)) {
      throw new Error(`SECURITY_VIOLATION: Agent "${permissions.agentId}" attempted forbidden action: ${forbidden}`);
    }
  }
  return true;
}

/**
 * Gate a live finding (T3MP3ST gateLiveFinding pattern).
 * PASS only when the claim is backed by real tool/machine output with a cryptographic SHA-256 receipt.
 */
export function gateLiveFinding(finding) {
  const reasons = [];
  const evidence = Array.isArray(finding?.evidence) ? finding.evidence : [];
  const toolEv = evidence.filter(
    (e) =>
      e &&
      TOOL_EVIDENCE_TYPES.has(e.type) &&
      typeof e.content === 'string' &&
      e.content.trim().length > 0 &&
      typeof e.sha256 === 'string' &&
      /^[a-f0-9]{64}$/i.test(e.sha256)
  );

  if (toolEv.length === 0) {
    reasons.push(
      'no tool-output evidence with SHA-256 receipt — provenance-strict requires a finding be backed by real tool output, not prose'
    );
  }

  if ((finding?.severity === 'critical' || finding?.severity === 'high') && evidence.length === 0) {
    reasons.push(`${finding.severity} severity asserted with zero evidence — severity must be backed by evidence`);
  }

  const provenance = toolEv.length > 0 ? 'tool' : evidence.length > 0 ? 'context' : 'none';

  // Cap effective severity if provenance is not 'tool'
  let effectiveSeverity = finding?.severity || 'info';
  if (provenance === 'none' && (effectiveSeverity === 'critical' || effectiveSeverity === 'high' || effectiveSeverity === 'medium')) {
    effectiveSeverity = 'low';
  } else if (provenance === 'context' && (effectiveSeverity === 'critical' || effectiveSeverity === 'high')) {
    effectiveSeverity = 'medium';
  }

  return {
    passed: reasons.length === 0,
    provenance,
    effectiveSeverity,
    reasons,
    checkedAt: Date.now(),
  };
}

