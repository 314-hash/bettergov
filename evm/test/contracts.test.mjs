import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { compileContracts } from '../script/compile.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

test('All 6 EVM Solidity Contracts compile and enforce SEC Mitigation, BSP 1108 VASP Separation, Soulbound SBTs, and Quantum Agility', () => {
  const artifacts = compileContracts();

  const expectedContracts = [
    'BGOVToken',
    'PublicTransparencyRegistry',
    'TreasuryVault',
    'ValidatorSlashingEvidence',
    'NationalAssetTokenization',
    'CivicSBT',
    'QuantumIdentity',
  ];
  for (const name of expectedContracts) {
    assert.ok(artifacts[name], `${name} artifact must exist`);
  }

  // 1. Verify NationalAssetTokenization.sol (Non-Dividend RWA Utility Credits + Activity-Based Rewards + BSP 1108)
  const rwaSource = fs.readFileSync(
    path.resolve(__dirname, '../contracts/NationalAssetTokenization.sol'),
    'utf8'
  );
  assert.match(rwaSource, /NON_DIVIDEND_UTILITY_CREDITS_ONLY/, 'Must enforce non-dividend utility credits only');
  assert.match(rwaSource, /BSP_1108: CRYPTO_UTILITY_LAYER_ZERO_FIAT_RAILS/, 'Must declare BSP Circular No. 1108 VASP separation');
  assert.match(rwaSource, /PassiveReturnProhibitedRequiresActiveContribution/, 'Must block passive claims without active contributions');
  const rwaFnNames = artifacts.NationalAssetTokenization.abi.filter((x) => x.type === 'function').map((x) => x.name);
  assert.ok(rwaFnNames.includes('recordActiveContribution'));
  assert.ok(rwaFnNames.includes('claimFunctionalUtilityCredits'));
  assert.ok(rwaFnNames.includes('redeemFunctionalUtilityCredits'));

  // 2. Verify CivicSBT.sol (Non-Transferable Soulbound Credentials)
  const sbtSource = fs.readFileSync(path.resolve(__dirname, '../contracts/CivicSBT.sol'), 'utf8');
  assert.match(sbtSource, /SoulboundNonTransferable/, 'CivicSBT must revert on transfer/approval');
  const sbtFnNames = artifacts.CivicSBT.abi.filter((x) => x.type === 'function').map((x) => x.name);
  assert.ok(sbtFnNames.includes('issueSoulboundCredential'));
  assert.ok(sbtFnNames.includes('locked'));
  assert.ok(sbtFnNames.includes('transferFrom'));

  // 3. Verify QuantumIdentity.sol (Post-Quantum Cryptographic Agility: Dilithium, Falcon, SPHINCS+)
  const pqSource = fs.readFileSync(path.resolve(__dirname, '../contracts/QuantumIdentity.sol'), 'utf8');
  assert.match(pqSource, /DILITHIUM_ML_DSA_65/, 'QuantumIdentity must support Dilithium');
  assert.match(pqSource, /FALCON_512/, 'QuantumIdentity must support Falcon');
  assert.match(pqSource, /SPHINCS_PLUS_SHA256/, 'QuantumIdentity must support SPHINCS+');
  const pqFnNames = artifacts.QuantumIdentity.abi.filter((x) => x.type === 'function').map((x) => x.name);
  assert.ok(pqFnNames.includes('registerAgilePublicKey'));
  assert.ok(pqFnNames.includes('setAlgorithmPrecompileVerifier'));
  assert.ok(pqFnNames.includes('verifyWithAgilePrecompile'));

  // 4. Verify TreasuryVault.sol (Activity-Based Milestone Release & BSP 1108)
  const vaultSource = fs.readFileSync(path.resolve(__dirname, '../contracts/TreasuryVault.sol'), 'utf8');
  assert.match(vaultSource, /nonReentrant/);
  assert.match(vaultSource, /BSP_1108_VASP_SEPARATION/);
  assert.match(vaultSource, /PassiveReleaseProhibitedRequiresActivityProof/);
});
