# Contributing to BETTERGOV Chain

1. **Preserve Protocol Integrity**: All changes to `chain/x/` or `evm/contracts/` must include deterministic unit and integration tests.
2. **Enforce Provenance & Honesty**: Every feature must be labeled accurately as `IMPLEMENTED`, `PARTIAL`, `MOCK`, or `PLANNED`. Never fabricate transaction hashes, block heights, or benchmark scores.
3. **Run the Full Verification Suite Before Opening a PR**:
   - `cd chain && go test -v ./...`
   - `cd evm && npm test`
   - `cd ai-security && npm test`
   - `cd backend && npm test`
   - `cd frontend && npm test`

