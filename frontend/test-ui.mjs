import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

test('Frontend includes all 11 required pages, BetterGov logo asset, and mandatory authenticity disclaimer', () => {
  const logoPath = path.join(__dirname, 'public/logo.png');
  assert.ok(fs.existsSync(logoPath), 'frontend/public/logo.png must exist');

  const appTsx = fs.readFileSync(path.join(__dirname, 'src/App.tsx'), 'utf8');
  const requiredPages = [
    'Home',
    'Network Overview',
    'Explorer',
    'Validators',
    'Staking',
    'Governance',
    'Treasury',
    'Public Records',
    'AI Security Center',
    'Developer Portal',
    'Documentation',
  ];
  for (const label of requiredPages) {
    assert.ok(appTsx.includes(label), `Frontend navigation must include page: ${label}`);
  }
  assert.match(appTsx, /UNVERIFIED_HASH_ANCHOR/);
  assert.match(appTsx, /PII GUARD/);
  assert.match(appTsx, /NationalAssetTokenization/);
  assert.match(appTsx, /CivicSBT/);
  assert.match(appTsx, /QuantumIdentity/);
  assert.match(appTsx, /BSP Circular No\. 1108/);
});

