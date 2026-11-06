import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { SecurityCoordinatorAgent } from './coordinator.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const coordinator = new SecurityCoordinatorAgent();

const contractsDir = path.resolve(__dirname, '../../evm/contracts');
const contracts = fs
  .readdirSync(contractsDir)
  .filter((f) => f.endsWith('.sol'))
  .map((f) => ({
    name: f,
    source: fs.readFileSync(path.join(contractsDir, f), 'utf8'),
  }));

const result = coordinator.runFullAuditMission({
  missionName: 'BETTERGOV Repository & Protocol Audit',
  validators: [
    { operatorAddress: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8', moniker: 'Sentinel-Alpha-Node', votingPower: 2500000, missedBlocks: 2 },
    { operatorAddress: '0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc', moniker: 'CivicTrust-Validator', votingPower: 2000000, missedBlocks: 0 },
    { operatorAddress: '0x90f79bf6eb2c4f870365e785982e1f101e93b906', moniker: 'Aegis-GovTech-Staking', votingPower: 1800000, missedBlocks: 1 },
    { operatorAddress: '0x15d34aaf54267db7d7c367839aaf71a00a2c6a65', moniker: 'OpenLedger-Guard', votingPower: 1500000, missedBlocks: 0 },
  ],
  network: {
    nodeId: 'bettergov-sentry-01',
    peers: [
      { remoteIp: '172.16.10.2', isOutbound: true },
      { remoteIp: '192.168.40.8', isOutbound: true },
    ],
  },
  contracts,
});

const outPath = path.resolve(__dirname, '../reports/latest-audit-report.json');
fs.writeFileSync(outPath, JSON.stringify(result.report, null, 2));
console.log(`[BETTERGOV AI Security] 9-Agent Audit Complete. Verified Findings: ${result.verifiedFindingsCount}. Report saved to ${outPath}`);

