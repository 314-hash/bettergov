/**
 * BETTERGOV AI Security — Evidence Vault
 * Inspired by T3MP3ST (src/evidence/index.ts)
 *
 * Features:
 * - Semantic title normalization & deduplication across parallel agents
 * - Mandatory secret & private key redaction before storage or reporting
 * - Gate-owned verification (callers cannot self-stamp verifiedAt without passing gateLiveFinding)
 */

import crypto from 'node:crypto';
import { gateLiveFinding } from '../verification/gate.mjs';

const DEDUP_STOP_WORDS = new Set([
  'the', 'a', 'an', 'in', 'of', 'on', 'at', 'with', 'for', 'from', 'via', 'and', 'or', 'to',
  'exposed', 'discovered', 'detected', 'found', 'present', 'available', 'missing', 'enabled',
  'disabled', 'vulnerable', 'issues', 'issue', 'problem', 'warning', 'warning:',
  'disclosure', 'disclosed', 'leak', 'leaked', 'revealed', 'identified', 'located', 'anomaly',
]);

const SECRET_PATTERNS = [
  /(?:private[_-]?key|mnemonic|seed[_-]?phrase|secret|api[_-]?key|authorization)\s*[:=]\s*["']?[A-Za-z0-9+/=_\-]{12,}["']?/gi,
  /\b0x[a-fA-F0-9]{64}\b(?=.*(?:priv|key|secret))/gi,
];

/**
 * Redact any raw credential or private key material from evidence or text.
 */
export function redactSecrets(input) {
  if (typeof input !== 'string') return '';
  let sanitized = input;
  for (const pat of SECRET_PATTERNS) {
    sanitized = sanitized.replace(pat, '[REDACTED_SECRET_CAPTURED=true]');
  }
  return sanitized;
}

export function computeEvidenceSha256(content) {
  return crypto.createHash('sha256').update(String(content)).digest('hex');
}

export class EvidenceVault {
  constructor() {
    this.findings = new Map();
  }

  static normalizeTitle(title) {
    return String(title || '')
      .toLowerCase()
      .replace(/[()]/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
      .split(' ')
      .filter((w) => w && !DEDUP_STOP_WORDS.has(w))
      .map((w) => {
        if (w.length > 4 && /s$/.test(w) && !/(ss|us|is|as|os)$/.test(w)) {
          return w.slice(0, -1);
        }
        return w;
      })
      .join(' ');
  }

  dedupKeyOf(finding) {
    const normTitle = EvidenceVault.normalizeTitle(finding.title);
    const target = finding.targetId || '?';
    return `${normTitle}@${target}`;
  }

  addFinding(finding) {
    const id = finding.id || crypto.randomUUID();
    const sanitizedEvidence = (Array.isArray(finding.evidence) ? finding.evidence : []).map((ev) => {
      const cleanContent = redactSecrets(ev.content || '');
      return {
        ...ev,
        content: cleanContent,
        sha256: ev.sha256 || computeEvidenceSha256(cleanContent),
      };
    });

    const stored = {
      ...finding,
      id,
      description: redactSecrets(finding.description || ''),
      assertedSeverity: finding.severity || 'info',
      severity: finding.severity || 'info',
      evidence: sanitizedEvidence,
      discoveredAt: finding.discoveredAt || Date.now(),
      // Verification is strictly gate-owned: strip any caller-supplied verified stamp
      verifiedAt: undefined,
      verifyGate: undefined,
    };

    const key = this.dedupKeyOf(stored);
    for (const existing of this.findings.values()) {
      if (this.dedupKeyOf(existing) === key) {
        for (const ev of stored.evidence) {
          if (!existing.evidence.some((e) => e.type === ev.type && e.sha256 === ev.sha256)) {
            existing.evidence.push(ev);
          }
        }
        return structuredClone(existing);
      }
    }

    this.findings.set(stored.id, stored);
    return structuredClone(stored);
  }

  verifyFinding(findingId) {
    const finding = this.findings.get(findingId);
    if (!finding) return undefined;

    const gate = gateLiveFinding(finding);
    finding.verifyGate = gate;
    finding.severity = gate.effectiveSeverity;

    if (gate.passed) {
      finding.verifiedAt = Date.now();
    } else {
      delete finding.verifiedAt;
    }
    return structuredClone(finding);
  }

  getAllFindings() {
    return Array.from(this.findings.values()).map((f) => structuredClone(f));
  }

  getVerifiedFindings() {
    return this.getAllFindings().filter((f) => f.verifyGate?.passed === true && f.verifiedAt !== undefined);
  }
}

