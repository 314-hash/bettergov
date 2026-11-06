import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { createAppHandler } from './server.mjs';

test('Backend API serves explorer overview, AI security status, and enforces PII Guard on transparency anchor', async () => {
  const server = http.createServer(createAppHandler());
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  const baseUrl = `http://127.0.0.1:${port}`;

  try {
    // 1. Health check
    const healthRes = await fetch(`${baseUrl}/api/v1/health`);
    assert.equal(healthRes.status, 200);
    const health = await healthRes.json();
    assert.equal(health.status, 'ok');

    // 2. Explorer overview
    const ovRes = await fetch(`${baseUrl}/api/v1/explorer/overview`);
    assert.equal(ovRes.status, 200);
    const ov = await ovRes.json();
    assert.equal(ov.metrics.nativeSymbol, 'BGOV');
    assert.equal(ov.validators.length, 4);

    // 3. Reject PII in transparency anchor
    const piiRes = await fetch(`${baseUrl}/api/v1/transparency/anchor`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title: 'Private Record for john.doe@example.com',
        agency: 'DOF',
        sha256Hash: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
      }),
    });
    assert.equal(piiRes.status, 400);

    // 4. Accept valid hash anchor and verify UNVERIFIED_HASH_ANCHOR + mandatory disclaimer
    const validRes = await fetch(`${baseUrl}/api/v1/transparency/anchor`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title: 'Municipal Roadway Audit Report Q3',
        agency: 'DPWH-REGION-4',
        sha256Hash: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
        offChainUri: 'ipfs://bafybeiroadwayauditq3',
      }),
    });
    assert.equal(validRes.status, 201);
    const rec = await validRes.json();
    assert.equal(rec.verificationStatus, 'UNVERIFIED_HASH_ANCHOR');
    assert.match(rec.disclaimer, /DOES NOT AUTOMATICALLY VALIDATE/);
  } finally {
    server.close();
  }
});

