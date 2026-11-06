# Security Policy

## 1. Scope & Non-Negotiable Principles
- **No Unaudited Mainnet Deployment**: BETTERGOV is currently in active development and testnet readiness. Do not deploy to mainnet or custody real public assets until an independent security review is completed.
- **Strict AI Agent Sandboxing**: Agents in `ai-security/` operate under immutable read-only permissions (`canChangeConsensus: false`, `canAccessValidatorPrivateKeys: false`, `canExecuteArbitraryTransactions: false`, `canSeizeUserFunds: false`).
- **Private Validator Admin RPC**: Validator management endpoints (`/admin/*` on `:26657`) enforce loopback IP checks (`127.0.0.1` / `::1`) and must never be exposed to the public internet.
- **No On-Chain PII**: Never submit cleartext Personally Identifiable Information (PII) to `x/transparency` or `PublicTransparencyRegistry.sol`.

## 2. Reporting a Vulnerability
Please report security vulnerabilities privately via GitHub Security Advisories rather than opening a public issue. Include a reproducible test case or tool output receipt.

