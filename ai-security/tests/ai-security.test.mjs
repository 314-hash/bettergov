import test from 'node:test';
import assert from 'node:assert/strict';
import { SecurityCoordinatorAgent } from '../orchestration/coordinator.mjs';
import { assertPermissionBoundary, createControlledPermissions } from '../verification/gate.mjs';
import { handleMcpRequest } from '../orchestration/mcp-server.mjs';

test('All 9 AI Security Agents initialize with strict read-only permissions and zero consensus/key/fund capabilities', () => {
  const coordinator = new SecurityCoordinatorAgent();
  const agents = coordinator.getAllAgents();

  assert.equal(agents.length, 9, 'Must instantiate all 9 required security agents');

  for (const agent of agents) {
    assert.equal(agent.permissions.canChangeConsensus, false, `${agent.name} must not change consensus`);
    assert.equal(agent.permissions.canAccessValidatorPrivateKeys, false, `${agent.name} must not access validator private keys`);
    assert.equal(agent.permissions.canExecuteArbitraryTransactions, false, `${agent.name} must not execute arbitrary transactions`);
    assert.equal(agent.permissions.canSeizeUserFunds, false, `${agent.name} must not seize user funds`);

    // Verify forbidden actions throw immediately
    assert.throws(
      () => assertPermissionBoundary(agent.permissions, 'CHANGE_CONSENSUS_PARAMS'),
      /SECURITY_VIOLATION/
    );
    assert.throws(
      () => assertPermissionBoundary(agent.permissions, 'ACCESS_VALIDATOR_PRIVATE_KEYS'),
      /SECURITY_VIOLATION/
    );
    assert.throws(
      () => assertPermissionBoundary(agent.permissions, 'SEIZE_USER_FUNDS'),
      /SECURITY_VIOLATION/
    );
  }
});

test('Live Verification Gate enforces tool-backed provenance and blocks unverified prose assertions', () => {
  const coordinator = new SecurityCoordinatorAgent();
  const vault = coordinator.vault;

  // 1. Add an unbacked model-prose assertion claiming Critical severity -> must fail verification gate and be downgraded to 'low'
  const unbacked = vault.addFinding({
    title: 'Hypothetical Validator Exploit Without Tool Output',
    description: 'Model thinks there might be an issue here.',
    severity: 'critical',
    targetId: 'validator-1',
    operatorId: 'agent-validator-guardian',
    evidence: [],
  });
  const checkedUnbacked = vault.verifyFinding(unbacked.id);
  assert.equal(checkedUnbacked.verifyGate.passed, false, 'Unbacked prose finding must NOT pass verification gate');
  assert.equal(checkedUnbacked.verifyGate.provenance, 'none');
  assert.equal(checkedUnbacked.severity, 'low', 'Unbacked critical assertion must be capped to low severity');
  assert.equal(checkedUnbacked.verifiedAt, undefined);

  // 2. Run full 9-agent mission with real tool inputs and verify tool-backed findings pass the gate
  const missionOut = coordinator.runFullAuditMission({
    missionName: 'Unit Test Security Sweep',
    validators: [
      { operatorAddress: '0xval1', moniker: 'AlphaNode', votingPower: 5000, missedBlocks: 48 },
      { operatorAddress: '0xval2', moniker: 'BetaNode', votingPower: 5000, missedBlocks: 0 },
    ],
    network: {
      nodeId: 'sentry-01',
      peers: [{ remoteIp: '10.0.1.5', isOutbound: false }],
    },
    contracts: [
      {
        name: 'VulnerableMock',
        source: 'contract VulnerableMock { function withdraw() external { require(tx.origin == msg.sender); msg.sender.call{value: 1 ether}(""); } }',
      },
    ],
    transactions: [
      { hash: '0xabc123', from: '0xtreasury', to: '0xext', valueBgov: 250000, blockNumber: 12 },
    ],
    proposals: [
      { id: 7, proposer: '0xprop', paramChanges: [{ subspace: 'mint', key: 'community_tax_bps', value: '4500' }] },
    ],
  });

  assert.ok(missionOut.verifiedFindingsCount >= 5, 'Expected tool-backed findings from Validator, Network, Contract, Tx, and Gov agents');
  assert.match(missionOut.report.markdown, /BETTERGOV AI Security Audit Report/);
});

test('Evidence Vault deduplicates parallel findings and redacts leaked secret keys', () => {
  const coordinator = new SecurityCoordinatorAgent();
  const vault = coordinator.vault;

  const f1 = vault.addFinding({
    title: 'Exposed Sensitive Path in Validator Node',
    description: 'Found private_key=0xabcdef1234567890abcdef1234567890 in log',
    severity: 'medium',
    targetId: 'node-alpha',
    operatorId: 'agent-1',
    evidence: [{ type: 'log', content: 'private_key=0xabcdef1234567890abcdef1234567890' }],
  });

  // Duplicate semantic title ("Sensitive Paths Discovered on Validator Node") on same target merges instead of duplicating
  const f2 = vault.addFinding({
    title: 'Sensitive Paths Discovered on Validator Node',
    description: 'Second agent noticed the same path',
    severity: 'medium',
    targetId: 'node-alpha',
    operatorId: 'agent-2',
    evidence: [{ type: 'log', content: 'additional log line' }],
  });

  assert.equal(f1.id, f2.id, 'Deduplication must merge semantic duplicate findings on the same target');
  assert.doesNotMatch(f2.description, /0xabcdef1234567890abcdef1234567890/, 'Raw private key must be redacted');
  assert.match(f2.description, /REDACTED_SECRET_CAPTURED=true/);
});

test('MCP Server rejects shell metacharacters in target parameter', () => {
  const badRes = handleMcpRequest({
    jsonrpc: '2.0',
    id: 99,
    method: 'tools/call',
    params: {
      name: 'bettergov_security_audit',
      arguments: { target: 'bettergov-devnet-1; rm -rf /' },
    },
  });
  assert.equal(badRes.result.isError, true, 'MCP server must reject shell metacharacters');
});

