# T3MP3ST Architectural Reference Evaluation for BETTERGOV AI Security

## 1. Executive Summary & Repository Metadata

- **Reference Repository**: `https://github.com/elder-plinius/T3MP3ST.git`
- **License**: AGPL-3.0
- **Core Purpose**: Multi-agent security framework with strict provenance gating, priority-driven mission control, evidence deduplication/redaction, MCP/HTTP interfaces, and keyless/local LLM support.
- **Architectural Role in BETTERGOV**: T3MP3ST is **not** a blockchain consensus or execution framework. BETTERGOV implements its Layer 1 consensus, native Proof-of-Stake, governance, public transparency modules, and EVM execution natively in Go (`chain/`) and Solidity (`evm/`), while adopting T3MP3ST's **evidence-first multi-agent orchestration and provenance verification architecture** for the independent defensive subsystem in `ai-security/`.

---

## 2. Detailed Subsystem Evaluation of T3MP3ST

### 2.1 Multi-Agent Orchestration (`src/general/index.ts`, `src/index.ts`)
- **Op General (`General` class)**: Serves as a strategic planner above the tactical `TempestCommand` execution harness. Given a high-level `Directive`, it produces an `OpPlan` containing:
  - Prioritized `targets` and `objectives`
  - `huntLanes` mapped to domain families (`MissionFamily`)
  - `authorityReceipts` required before active operations
  - `evidenceContract` specifying minimum confidence, `provenanceFloor` (`weak | context | tool | replayable`), and retest rules
  - `workOrders` with explicit `hypothesis`, `safeProbe`, `expectedSignal`, `evidenceArtifact`, and `falsifier`
  - Adversarial self-critique (`OpPlanCritique`) and `missionGate` (`ready | degraded | hold`).
- **BETTERGOV Adaptation**: `SecurityCoordinator` in `ai-security/orchestration/` generates defensive audit plans (`AuditMissionPlan`) with explicit falsifiers, evidence contracts, and read-only scope boundaries across validators, P2P nodes, smart contracts, transactions, and governance proposals.

### 2.2 Mission Management & Task Queue (`src/mission/index.ts`)
- **`MissionControl` & `TaskQueue`**:
  - Event-driven lifecycle (`mission:created`, `mission:started`, `mission:paused`, `mission:completed`, `mission:aborted`, `task:assigned`, `task:completed`, `task:failed`).
  - Priority-sorted task queue (`b.priority - a.priority`) so tool-verified follow-up leads preempt lower-priority static seed tasks.
  - **`RulesOfEngagement` (`RoE`)**: Defines `scope`, `excludedTargets`, `allowedTechniques`, `forbiddenTechniques`, `maxDetectionEvents`, and `requireManualApproval`.
- **BETTERGOV Adaptation**: `MissionControl` enforces strict defensive Rules of Engagement where every AI agent operates with `canChangeConsensus: false`, `canAccessValidatorKeys: false`, `canExecuteArbitraryTx: false`, and `canSeizeFunds: false`.

### 2.3 Operator Architecture (`src/types/index.ts`, `src/operators/`)
- **T3MP3ST Archetypes**: `recon`, `scanner`, `exploiter`, `infiltrator`, `exfiltrator`, `ghost`, `coordinator`, `analyst`.
- **Operator State Machine**: `idle -> tasked -> executing -> cooldown` with per-operator metrics (`completedTasks`, `failedTasks`, `findingsCount`, `lastActivityTime`).
- **BETTERGOV Adaptation**: Replaces offensive kill-chain operators with 9 specialized defensive blockchain security agents:
  1. `ValidatorGuardian` (monitors validator uptime, missed-block windows, commission changes, voting power concentration)
  2. `NetworkSentinel` (monitors P2P peer diversity, inbound/outbound ratios, RPC latency, eclipse indicators)
  3. `ContractAuditor` (analyzes EVM bytecode and Solidity AST/source for reentrancy, access control, unchecked low-level calls, centralizations)
  4. `TransactionAnalyst` (identifies wash trading, abnormal gas spikes, treasury drain patterns, replay attempts)
  5. `GovernanceAnalyst` (detects quorum manipulation, last-block flash-stake voting anomalies, risky parameter changes)
  6. `EvidenceCollector` (captures deterministic RPC responses, transaction receipts, state hashes, and strips sensitive secrets)
  7. `VerificationAgent` (enforces the Live Verification Gate on every finding before promotion)
  8. `SecurityCoordinator` (orchestrates missions, task queues, and cross-agent correlation)
  9. `ReportGenerator` (compiles reproducible JSON and Markdown security reports with provenance receipts)

### 2.4 Evidence Vault (`src/evidence/index.ts`)
- **Semantic Deduplication**: Normalizes finding titles by stripping stop words (`exposed`, `discovered`, `detected`, `the`, `in`, etc.) and singularizing plural nouns (`dedupKeyOf(f) = "${normalizedTitle}@${target}"`). Duplicate findings from parallel agents merge their evidence arrays and retain the strongest verification gate state rather than inflating finding counts.
- **Secret Redaction (`redactCredential`)**: Secrets are stripped from all outward-facing API responses, LLM contexts, and reports (`secretCaptured: boolean` replaces raw secret strings).
- **Gate-Owned Verification**: `updateFinding()` explicitly deletes `verifiedAt` and `verifyGate` from caller-supplied updates so callers cannot self-stamp a finding as verified without passing `verifyFinding()`.

### 2.5 Verification Systems & Provenance Gate (`src/evidence/gate.ts`, `docs/VERIFIED_PROVENANCE.md`)
- **`gateLiveFinding(finding)`**:
  - Inspects `finding.evidence` for machine/tool evidence types (`output`, `command`, `response`, `request`, `log`, `file`) with non-empty content.
  - Classifies provenance as `'tool'`, `'context'`, or `'none'`.
  - Refuses to mark a finding as `verified` if `toolEv.length === 0` (`"no tool-output evidence — provenance-strict requires a finding be backed by real tool output, not prose"`).
  - Caps asserted `critical`/`high` severities when unverified (`none -> low`, `context -> medium`, `tool -> unchanged`).
- **BETTERGOV Adaptation**: Implemented directly in `ai-security/verification/gate.ts`. No AI security finding in BETTERGOV can be marked `VERIFIED` or trigger a high-severity governance alert unless backed by cryptographic or RPC tool evidence (block header hash, tx receipt hash, bytecode AST match, or slashing evidence record).

### 2.6 REST API & MCP Integration (`src/server.ts`, `src/mcp-server.ts`)
- **HTTP API (`src/server.ts`)**: Binds to `127.0.0.1` by default with loopback Host-header and Origin guards.
- **MCP Server (`src/mcp-server.ts`)**: Uses `@modelcontextprotocol/sdk` over stdio, validates targets against a strict regex (`TARGET_RE = /^[A-Za-z0-9._:-]+$/`), rejects NUL bytes, and invokes allowlisted binaries via `execFile` (no shell interpretation).
- **BETTERGOV Adaptation**: `ai-security/orchestration/mcp-server.ts` and `backend/src/server.ts` enforce strict input validation, loopback admin boundaries, and read-only tool invocation.

### 2.7 Local AI Integration (`src/llm/index.ts`, `src/config/index.ts`)
- Supports local OpenAI-compatible endpoints (Ollama `http://localhost:11434/api`, LM Studio, vLLM), connected local coding-agent CLIs, or deterministic rule/tool-backed fallback execution when no external API key is configured.
- **BETTERGOV Adaptation**: `ai-security/` works out-of-the-box in keyless deterministic + local-LLM mode (`OLLAMA_BASE_URL` or built-in deterministic analyzer) so validator operators never have to send sensitive telemetry to third-party cloud APIs.

### 2.8 Testing & Reproducible Reporting (`scripts/verify-claims.mjs`)
- Every metric and security finding is recomputed from committed artifacts and tool outputs rather than prose assertions.
- **BETTERGOV Adaptation**: Both `chain/` (`go test ./...`), `evm/` (contract test suite), and `ai-security/` (`npm test`) run automated verification suites checking provenance gates, permission boundaries, consensus slashing, and governance invariants.

