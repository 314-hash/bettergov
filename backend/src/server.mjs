import http from 'node:http';
import { BetterGovIndexer } from '../../indexer/src/indexer.mjs';
import { ExplorerService } from '../../explorer/src/explorer-service.mjs';
import { SecurityCoordinatorAgent } from '../../ai-security/orchestration/coordinator.mjs';

const EMAIL_RE = /[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}/;
const SSN_RE = /\b\d{3}-\d{2}-\d{4}\b/;
const SHA256_RE = /^[a-fA-F0-9]{64}$/;

export function createBackendContext() {
  const indexer = new BetterGovIndexer();
  const explorer = new ExplorerService(indexer);
  const aiCoordinator = new SecurityCoordinatorAgent();
  return { indexer, explorer, aiCoordinator };
}

export function createAppHandler(ctx = createBackendContext()) {
  return async (req, res) => {
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

    if (req.method === 'OPTIONS') {
      res.writeHead(200);
      res.end();
      return;
    }

    const url = new URL(req.url || '/', 'http://127.0.0.1');

    if (req.method === 'GET' && url.pathname === '/api/v1/health') {
      return sendJson(res, 200, {
        status: 'ok',
        service: 'BETTERGOV API & Explorer Gateway',
        version: '1.0.0',
        timestamp: new Date().toISOString(),
      });
    }

    if (req.method === 'GET' && url.pathname === '/api/v1/explorer/overview') {
      return sendJson(res, 200, {
        metrics: ctx.explorer.getOverviewMetrics(),
        blocks: ctx.explorer.getBlocks(),
        validators: ctx.explorer.getValidators(),
        contracts: ctx.explorer.getSmartContracts(),
        proposals: ctx.explorer.getProposals(),
        transparencyRecords: ctx.explorer.getTransparencyRecords(),
      });
    }

    if (req.method === 'GET' && url.pathname === '/api/v1/ai-security/status') {
      const audit = ctx.aiCoordinator.runFullAuditMission({
        missionName: 'Live Telemetry Security Check',
        validators: ctx.explorer.getValidators(),
      });
      return sendJson(res, 200, audit);
    }

    if (req.method === 'POST' && url.pathname === '/api/v1/transparency/anchor') {
      const body = await readJsonBody(req);
      const { id, category, title, agency, sha256Hash, offChainUri } = body || {};

      if (EMAIL_RE.test(title || '') || SSN_RE.test(title || '') || EMAIL_RE.test(agency || '')) {
        return sendJson(res, 400, {
          error: 'REJECTED_PII_DETECTED: Sensitive personal information must remain off-chain. Submit cryptographic SHA-256 hashes only.',
        });
      }
      const cleanHash = String(sha256Hash || '').replace(/^0x/i, '');
      if (!SHA256_RE.test(cleanHash)) {
        return sendJson(res, 400, {
          error: 'INVALID_SHA256: sha256Hash must be a 64-character hexadecimal SHA-256 digest.',
        });
      }

      const record = {
        id: id || `DOC-${Date.now()}`,
        category: category || 'PUBLIC_DOCUMENT',
        title: String(title || 'Untitled Record'),
        agency: String(agency || 'PUBLIC_AGENCY'),
        sha256Hash: cleanHash.toLowerCase(),
        offChainUri: String(offChainUri || ''),
        verificationStatus: 'UNVERIFIED_HASH_ANCHOR',
        verifiedByAuditor: null,
        disclaimer:
          'ON-CHAIN RECORD PROVES CRYPTOGRAPHIC INTEGRITY AND TIMESTAMP ONLY. RECORDING A HASH ON BETTERGOV DOES NOT AUTOMATICALLY VALIDATE THE UNDERLYING DOCUMENT OR GOVERNMENT CLAIM UNLESS INDEPENDENTLY VERIFIED BY AUTHORIZED AUDITORS.',
        blockHeight: ctx.indexer.lastSyncedHeight,
        timestamp: new Date().toISOString(),
      };
      ctx.indexer.transparencyRecords.unshift(record);
      return sendJson(res, 201, record);
    }

    return sendJson(res, 404, { error: 'Endpoint not found' });
  };
}

function sendJson(res, status, payload) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(payload, null, 2));
}

function readJsonBody(req) {
  return new Promise((resolve) => {
    let data = '';
    req.on('data', (chunk) => (data += chunk));
    req.on('end', () => {
      try {
        resolve(JSON.parse(data || '{}'));
      } catch {
        resolve({});
      }
    });
  });
}

if (process.argv[1] && process.argv[1].endsWith('server.mjs')) {
  const port = Number(process.env.PORT || 4000);
  const server = http.createServer(createAppHandler());
  server.listen(port, '127.0.0.1', () => {
    console.log(`[BETTERGOV Backend] Listening on http://127.0.0.1:${port}`);
  });
}

