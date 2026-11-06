package tests

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bettergov/chain/app"
	"github.com/bettergov/chain/x/bank"
	"github.com/bettergov/chain/x/evm"
	"github.com/bettergov/chain/x/gov"
	"github.com/bettergov/chain/x/slashing"
	"github.com/bettergov/chain/x/staking"
	"github.com/bettergov/chain/x/transparency"
)

func setupTestChain(t *testing.T) *app.BetterGovApp {
	t.Helper()
	gen := app.DefaultDevnetGenesis()
	chainApp, err := app.NewBetterGovApp(gen)
	if err != nil {
		t.Fatalf("failed to create BetterGovApp: %v", err)
	}
	return chainApp
}

// 1. Native Proof-of-Stake Consensus, Delegation, Redelegation & Unbonding Lifecycle Test
func TestNativeProofOfStakeLifecycle(t *testing.T) {
	chain := setupTestChain(t)

	updates := chain.Staking.GetValidatorUpdates()
	if len(updates) != 4 {
		t.Fatalf("expected 4 active genesis validators, got %d", len(updates))
	}
	// Verify deterministic sorting by voting power descending
	if updates[0].VotingPower < updates[1].VotingPower {
		t.Fatalf("validators not sorted by voting power descending: %d < %d", updates[0].VotingPower, updates[1].VotingPower)
	}

	delegator := "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
	valAlpha := "0x70997970c51812dc3a010c7d01b50e0d17dc79c8"
	valBeta := "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc"
	valGamma := "0x90f79bf6eb2c4f870365e785982e1f101e93b906"

	delegateAmt := bank.BGOVToAbgov(500_000)
	shares, err := chain.Staking.Delegate(delegator, valAlpha, delegateAmt)
	if err != nil {
		t.Fatalf("Delegate failed: %v", err)
	}
	if shares.Cmp(delegateAmt) != 0 {
		t.Fatalf("expected 1:1 initial shares %s, got %s", delegateAmt, shares)
	}

	// Finalize block to mint provisions and distribute staking rewards
	now := time.Now().UTC()
	_, err = chain.FinalizeBlock(now, nil, nil)
	if err != nil {
		t.Fatalf("FinalizeBlock failed: %v", err)
	}

	// Withdraw delegator rewards and verify positive reward payout
	reward, err := chain.Staking.WithdrawDelegatorRewards(delegator, valAlpha)
	if err != nil {
		t.Fatalf("WithdrawDelegatorRewards failed: %v", err)
	}
	if reward.Sign() <= 0 {
		t.Fatalf("expected positive staking reward after block finalization, got %s", reward.String())
	}

	// Test Redelegation from valAlpha -> valBeta
	redelShares := bank.BGOVToAbgov(200_000)
	_, _, err = chain.Staking.BeginRedelegate(delegator, valAlpha, valBeta, redelShares, chain.Height, now)
	if err != nil {
		t.Fatalf("BeginRedelegate failed: %v", err)
	}

	// Verify transitive redelegation lock (valBeta -> valGamma while valAlpha -> valBeta is active must fail)
	_, _, err = chain.Staking.BeginRedelegate(delegator, valBeta, valGamma, redelShares, chain.Height, now)
	if err != staking.ErrTransitiveRedelegation {
		t.Fatalf("expected ErrTransitiveRedelegation, got %v", err)
	}

	// Test Undelegation and time-locked completion
	undelShares := bank.BGOVToAbgov(100_000)
	balBefore := chain.Bank.GetBalance(delegator)
	_, completionTime, err := chain.Staking.Undelegate(delegator, valAlpha, undelShares, chain.Height, now)
	if err != nil {
		t.Fatalf("Undelegate failed: %v", err)
	}

	// Finalize block AFTER completionTime and verify tokens returned to delegator
	_, err = chain.FinalizeBlock(completionTime.Add(2*time.Second), nil, nil)
	if err != nil {
		t.Fatalf("FinalizeBlock after unbonding failed: %v", err)
	}
	balAfter := chain.Bank.GetBalance(delegator)
	expectedBal := new(big.Int).Add(balBefore, undelShares)
	if balAfter.Cmp(expectedBal) != 0 {
		t.Fatalf("expected balance %s after unbonding maturity, got %s", expectedBal, balAfter)
	}
}

// 2. Slashing Tests: Downtime Window Jailing, Unjailing, and Double-Sign Permanent Tombstone
func TestSlashingDowntimeAndDoubleSign(t *testing.T) {
	chain := setupTestChain(t)

	valBetaOp := "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc"
	valBeta, err := chain.Staking.GetValidator(valBetaOp)
	if err != nil {
		t.Fatalf("GetValidator failed: %v", err)
	}
	initialTokens := new(big.Int).Set(valBeta.Tokens)

	// Simulate 51 missed blocks out of 100-block window (MinSignedPerWindow = 50%)
	baseTime := time.Now().UTC()
	absentMap := map[string]bool{valBeta.ConsensusAddress: true}
	for i := 1; i <= 51; i++ {
		_, err := chain.FinalizeBlock(baseTime.Add(time.Duration(i)*time.Second), nil, absentMap)
		if err != nil {
			t.Fatalf("FinalizeBlock %d failed: %v", i, err)
		}
	}

	valBetaAfter, _ := chain.Staking.GetValidator(valBetaOp)
	if !valBetaAfter.Jailed {
		t.Fatalf("expected validator to be jailed after exceeding downtime window")
	}
	if valBetaAfter.Tokens.Cmp(initialTokens) >= 0 {
		t.Fatalf("expected validator tokens to be slashed for downtime: before=%s after=%s", initialTokens, valBetaAfter.Tokens)
	}

	// Attempting to unjail before DowntimeJailDuration expires must fail
	if err := chain.Slashing.Unjail(valBetaOp, baseTime.Add(100*time.Second)); err != slashing.ErrStillJailed {
		t.Fatalf("expected ErrStillJailed when unjailing early, got %v", err)
	}

	// Unjailing after jail expiry succeeds
	if err := chain.Slashing.Unjail(valBetaOp, baseTime.Add(700*time.Second)); err != nil {
		t.Fatalf("expected Unjail to succeed after jail duration, got %v", err)
	}

	// Now test Double-Sign Evidence on Validator Gamma -> must slash 5% and permanently Tombstone
	valGammaOp := "0x90f79bf6eb2c4f870365e785982e1f101e93b906"
	valGamma, _ := chain.Staking.GetValidator(valGammaOp)
	ev := slashing.DoubleSignEvidence{
		ConsensusAddress: valGamma.ConsensusAddress,
		Height:           10,
		Round:            0,
		BlockHashA:       "0xaaaa1111",
		BlockHashB:       "0xbbbb2222",
		EvidenceTime:     baseTime,
	}
	slashEv, err := chain.Slashing.HandleDoubleSignEvidence(ev, chain.Height, baseTime)
	if err != nil {
		t.Fatalf("HandleDoubleSignEvidence failed: %v", err)
	}
	if !slashEv.Tombstoned {
		t.Fatalf("expected double-sign slash event to mark Tombstoned=true")
	}

	// Tombstoned validator can NEVER unjail even years later
	err = chain.Slashing.Unjail(valGammaOp, baseTime.Add(365*24*time.Hour))
	if err != staking.ErrValidatorTombstoned {
		t.Fatalf("expected ErrValidatorTombstoned on unjail attempt, got %v", err)
	}
}

// 3. Protocol Governance Tests: Proposal Deposit, Bonded Voting Power, Quorum, Veto Burn, and Execution
func TestGovernanceProposalsAndExecution(t *testing.T) {
	chain := setupTestChain(t)
	now := time.Now().UTC()

	proposer := "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
	val1 := "0x70997970c51812dc3a010c7d01b50e0d17dc79c8"
	val2 := "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc"
	recipient := "0x1111111111111111111111111111111111111111"

	spendAmt := bank.BGOVToAbgov(50_000)
	prop, err := chain.Gov.SubmitProposal(
		proposer,
		"BGP-1: National Open Procurement Infrastructure Grant",
		"Allocate 50,000 BGOV from Community Treasury to deploy open-source municipal procurement verifiers.",
		"ipfs://bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		gov.ProposalTypeTreasurySpend,
		bank.BGOVToAbgov(1000), // Meets MinDeposit immediately
		nil,
		&gov.TreasurySpendSpec{
			Recipient:   recipient,
			AmountAbgov: spendAmt,
			ProjectID:   "PROJ-2026-001",
		},
		nil,
		now,
	)
	if err != nil {
		t.Fatalf("SubmitProposal failed: %v", err)
	}
	if prop.Status != gov.StatusVotingPeriod {
		t.Fatalf("expected proposal in voting period, got %s", prop.Status)
	}

	// Vote YES with val1 and val2 (4.5M / 7.8M = 57.6% bonded power -> exceeds 33.4% quorum and 50% threshold)
	if _, err := chain.Gov.Vote(prop.ID, val1, gov.VoteOptionYes, "Verified civic milestone plan", now); err != nil {
		t.Fatalf("Vote val1 failed: %v", err)
	}
	if _, err := chain.Gov.Vote(prop.ID, val2, gov.VoteOptionYes, "Supports public transparency", now); err != nil {
		t.Fatalf("Vote val2 failed: %v", err)
	}

	// Advance block time past VotingEndTime to trigger EndBlocker execution
	_, err = chain.FinalizeBlock(prop.VotingEndTime.Add(5*time.Second), nil, nil)
	if err != nil {
		t.Fatalf("FinalizeBlock failed: %v", err)
	}

	executedProp, _ := chain.Gov.GetProposal(prop.ID)
	if executedProp.Status != gov.StatusExecuted {
		t.Fatalf("expected proposal status EXECUTED, got %s (log: %s)", executedProp.Status, executedProp.ExecutionLog)
	}

	recipientBal := chain.Bank.GetBalance(recipient)
	if recipientBal.Cmp(spendAmt) != 0 {
		t.Fatalf("expected treasury recipient balance %s, got %s", spendAmt, recipientBal)
	}

	// Test Vetoed Proposal: NoWithVeto >= 33.4% must burn the deposit
	burnedBefore := chain.Bank.GetTotalBurned()
	propVeto, err := chain.Gov.SubmitProposal(
		proposer,
		"BGP-2: Controversial Parameter Change",
		"Attempt to raise community tax to 50%",
		"ipfs://bafybeiveto",
		gov.ProposalTypeText,
		bank.BGOVToAbgov(1000),
		nil, nil, nil, now,
	)
	if err != nil {
		t.Fatalf("SubmitProposal 2 failed: %v", err)
	}
	_, _ = chain.Gov.Vote(propVeto.ID, val1, gov.VoteOptionNoWithVeto, "Unacceptable tax hike", now)
	_, _ = chain.Gov.Vote(propVeto.ID, val2, gov.VoteOptionNoWithVeto, "Vetoing proposal", now)

	_, _ = chain.FinalizeBlock(propVeto.VotingEndTime.Add(5*time.Second), nil, nil)
	vetoedProp, _ := chain.Gov.GetProposal(propVeto.ID)
	if vetoedProp.Status != gov.StatusVetoed {
		t.Fatalf("expected proposal status VETOED, got %s", vetoedProp.Status)
	}
	burnedAfter := chain.Bank.GetTotalBurned()
	if new(big.Int).Sub(burnedAfter, burnedBefore).Cmp(bank.BGOVToAbgov(1000)) != 0 {
		t.Fatalf("expected 1000 BGOV deposit burned on veto")
	}
}

// 4. Public Transparency Module Tests: PII Guard, Hash Validation, Disclaimers & Auditor Verification
func TestPublicTransparencyAndPIIGuard(t *testing.T) {
	chain := setupTestChain(t)
	now := time.Now().UTC()
	validHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Attempting to submit cleartext email or SSN in metadata must be rejected by PII Guard
	_, err := chain.Transparency.RecordDocument(
		"DOC-PII-FAIL",
		"Citizen Tax Filing for citizen@example.com",
		"BIR-01",
		validHash,
		"ipfs://bafy123",
		"0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
		1,
		now,
	)
	if err != transparency.ErrCleartextPIIDetected {
		t.Fatalf("expected ErrCleartextPIIDetected for email in title, got %v", err)
	}

	// Valid public document hash submission succeeds and starts as UNVERIFIED_HASH_ANCHOR with mandatory disclaimer
	doc, err := chain.Transparency.RecordDocument(
		"DOC-2026-001",
		"FY2026 National Infrastructure Audit Summary",
		"COA-PH",
		validHash,
		"ipfs://bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		"0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
		1,
		now,
	)
	if err != nil {
		t.Fatalf("RecordDocument failed: %v", err)
	}
	if doc.VerificationStatus != transparency.StatusUnverifiedHashAnchor {
		t.Fatalf("expected initial status UNVERIFIED_HASH_ANCHOR, got %s", doc.VerificationStatus)
	}
	if doc.AttestationDisclaimer == "" {
		t.Fatalf("mandatory attestation disclaimer missing")
	}

	// Unauthorized address cannot mark document as AUDITOR_VERIFIED
	_, err = chain.Transparency.VerifyDocumentByAuditor(
		"DOC-2026-001",
		"0x9999999999999999999999999999999999999999",
		validHash,
		"Unauthorized check",
		transparency.StatusAuditorVerified,
		now,
	)
	if err != transparency.ErrUnauthorizedAuditor {
		t.Fatalf("expected ErrUnauthorizedAuditor, got %v", err)
	}

	// Registered auditor successfully verifies the document
	verifiedDoc, err := chain.Transparency.VerifyDocumentByAuditor(
		"DOC-2026-001",
		"0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
		validHash,
		"Cross-checked SHA-256 against signed COA Gazette publication",
		transparency.StatusAuditorVerified,
		now,
	)
	if err != nil {
		t.Fatalf("VerifyDocumentByAuditor failed: %v", err)
	}
	if verifiedDoc.VerificationStatus != transparency.StatusAuditorVerified {
		t.Fatalf("expected status AUDITOR_VERIFIED, got %s", verifiedDoc.VerificationStatus)
	}
}

// 5. EVM Compatibility, Contract Deployment, Replay Protection & JSON-RPC / Admin Loopback Security Test
func TestEVMExecutionAndJSONRPCSecurity(t *testing.T) {
	chain := setupTestChain(t)
	sender := "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"

	// Deploy contract via EVMTransaction
	deployTx := &evm.EVMTransaction{
		ChainID:  31337,
		Nonce:    0,
		From:     sender,
		To:       "",
		Value:    big.NewInt(0),
		DataHex:  "0x6080604052348015600f57600080fd5b50603f80601d6000396000f3fe",
		GasLimit: 200000,
		GasPrice: big.NewInt(1_000_000_000),
	}
	_, err := chain.FinalizeBlock(time.Now().UTC(), []*evm.EVMTransaction{deployTx}, nil)
	if err != nil {
		t.Fatalf("contract deploy block failed: %v", err)
	}

	rc, ok := chain.EVM.GetReceipt(deployTx.Hash)
	if !ok || rc.ContractAddress == "" || rc.Status != 1 {
		t.Fatalf("expected valid contract deployment receipt, got %+v", rc)
	}

	// Replay attempt with same nonce (0) must be rejected
	_, err = chain.FinalizeBlock(time.Now().UTC(), []*evm.EVMTransaction{deployTx}, nil)
	if err == nil {
		t.Fatalf("expected replay transaction with duplicate nonce 0 to be rejected")
	}

	// Test JSON-RPC eth_chainId and eth_getCode via HTTP handler
	mux := chain.NewPublicMux()
	reqBody, _ := json.Marshal(app.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getCode",
		Params:  []json.RawMessage{json.RawMessage(`"` + rc.ContractAddress + `"`)},
		ID:      1,
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from JSON-RPC, got %d", w.Code)
	}

	// Test Private Admin RPC rejects non-loopback remote IP
	adminHandler := chain.NewPrivateAdminMux()
	extReq := httptest.NewRequest(http.MethodGet, "/admin/snapshot", nil)
	extReq.RemoteAddr = "203.0.113.42:54321"
	extW := httptest.NewRecorder()
	adminHandler.ServeHTTP(extW, extReq)
	if extW.Code != http.StatusForbidden {
		t.Fatalf("expected Private Admin RPC to return 403 Forbidden for external IP, got %d", extW.Code)
	}

	// Loopback IP succeeds
	localReq := httptest.NewRequest(http.MethodGet, "/admin/snapshot", nil)
	localReq.RemoteAddr = "127.0.0.1:54321"
	localW := httptest.NewRecorder()
	adminHandler.ServeHTTP(localW, localReq)
	if localW.Code != http.StatusOK {
		t.Fatalf("expected Private Admin RPC to return 200 OK for loopback IP, got %d", localW.Code)
	}
}

// 6. SEC Classification Mitigation (Activity-Based Staking Boosters) & BSP Circular No. 1108 Guard Test
func TestSECMitigationActivityBoostAndBSP1108Guard(t *testing.T) {
	chain := setupTestChain(t)

	// Verify BSP Circular No. 1108 guard blocks direct PHP fiat rail tags
	if err := bank.ValidateBSP1108CryptoUtilityOnly("PHP"); err != bank.ErrBSP1108FiatRailsProhibited {
		t.Fatalf("expected ErrBSP1108FiatRailsProhibited for PHP fiat rail, got %v", err)
	}
	if err := bank.ValidateBSP1108CryptoUtilityOnly("abgov"); err != nil {
		t.Fatalf("expected abgov crypto-utility denom to pass BSP 1108 check, got %v", err)
	}

	// Compare two delegators with identical bonded stake on the same validator:
	// Delegator A records active contributions (CLEAN_ENERGY + CROP_SUBMISSION), while Delegator B is passive.
	valAlpha := "0x70997970c51812dc3a010c7d01b50e0d17dc79c8"
	activeDelegator := "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
	passiveDelegator := "0x15d34aaf54267db7d7c367839aaf71a00a2c6a65"

	stakeAmt := bank.BGOVToAbgov(500_000)
	if _, err := chain.Staking.Delegate(activeDelegator, valAlpha, stakeAmt); err != nil {
		t.Fatalf("activeDelegator Delegate failed: %v", err)
	}
	if _, err := chain.Staking.Delegate(passiveDelegator, valAlpha, stakeAmt); err != nil {
		t.Fatalf("passiveDelegator Delegate failed: %v", err)
	}

	boostBps, err := chain.Staking.RecordActiveContribution(
		activeDelegator,
		valAlpha,
		"CLEAN_ENERGY_GENERATION",
		"0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
	)
	if err != nil || boostBps != 500 {
		t.Fatalf("expected 500 bps activity boost, got %d (%v)", boostBps, err)
	}

	_, err = chain.FinalizeBlock(time.Now().UTC(), nil, nil)
	if err != nil {
		t.Fatalf("FinalizeBlock failed: %v", err)
	}

	activeReward, _ := chain.Staking.WithdrawDelegatorRewards(activeDelegator, valAlpha)
	passiveReward, _ := chain.Staking.WithdrawDelegatorRewards(passiveDelegator, valAlpha)
	if activeReward.Cmp(passiveReward) <= 0 {
		t.Fatalf("expected active contributor reward (%s) > passive holder reward (%s)", activeReward, passiveReward)
	}
}

// 7. Unlimited Validators with BETTERGOV Institutional Roles & Live REST Registration Test
func TestUnlimitedValidatorsWithRolesAndLiveREST(t *testing.T) {
	chain := setupTestChain(t)
	stParams := chain.Staking.GetParams()
	if !stParams.UnlimitedValidators || stParams.MaxValidators != 0 {
		t.Fatalf("expected unlimited validator set (MaxValidators=0, UnlimitedValidators=true), got %+v", stParams)
	}

	pubMux := chain.NewPublicMux()
	createPayload := map[string]interface{}{
		"operator_address":          "0x9965507d1a55bcc2695c58ba16fb37d819b0a4dc",
		"consensus_pubkey_hex":      "5555555555555555555555555555555555555555555555555555555555555555",
		"moniker":                   "Visayas-Audit-Sentinel",
		"role":                      staking.RolePublicAuditorNode,
		"website":                   "https://visayas-audit.bettergov.org",
		"security_contact":          "sec-visayas-audit",
		"details":                   "Regional Fiscal Audit & Procurement Verification Node",
		"self_bond_bgov":            250000,
		"min_self_delegation_bgov":  50000,
		"commission_rate_bps":       500,
		"commission_max_rate_bps":   2000,
		"commission_max_change_bps": 100,
	}
	rawBody, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/cosmos/staking/v1beta1/create-validator", bytes.NewReader(rawBody))
	rec := httptest.NewRecorder()
	pubMux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from /cosmos/staking/v1beta1/create-validator, got %d: %s", rec.Code, rec.Body.String())
	}

	newVal, err := chain.Staking.GetValidator("0x9965507d1a55bcc2695c58ba16fb37d819b0a4dc")
	if err != nil {
		t.Fatalf("expected new validator to be registered, got err: %v", err)
	}
	if newVal.Role != staking.RolePublicAuditorNode {
		t.Fatalf("expected validator role %s, got %s", staking.RolePublicAuditorNode, newVal.Role)
	}
	if newVal.Status != staking.BondStatusBonded {
		t.Fatalf("expected new validator to immediately enter BOND_STATUS_BONDED in unlimited set, got %s", newVal.Status)
	}
	if len(chain.Staking.GetValidatorUpdates()) != 5 {
		t.Fatalf("expected 5 active validators in unlimited set, got %d", len(chain.Staking.GetValidatorUpdates()))
	}
}

// 6. Fuzz / Property Test: Bank Conservation of Supply and PII Guard Invariant
func FuzzPIIGuardAndBankTransferConservation(f *testing.F) {
	f.Add("Clean Highway Project Phase 2", int64(1500))
	f.Add("Contact john.doe@gov.example.org for bid", int64(50000))
	f.Add("Taxpayer ID 123-45-6789 record", int64(1))

	f.Fuzz(func(t *testing.T, metadata string, transferAmount int64) {
		b := bank.NewKeeper()
		initial := bank.BGOVToAbgov(1_000_000)
		_ = b.MintTokens("0xaaaa", initial)

		supplyBefore := b.GetTotalSupply()
		if transferAmount > 0 && transferAmount <= 1_000_000 {
			amt := bank.BGOVToAbgov(transferAmount)
			if err := b.Transfer("0xaaaa", "0xbbbb", amt); err != nil {
				t.Fatalf("unexpected transfer error: %v", err)
			}
		}
		supplyAfter := b.GetTotalSupply()
		// Property: Transfers never create or destroy total supply
		if supplyBefore.Cmp(supplyAfter) != 0 {
			t.Fatalf("supply conservation violated: before=%s after=%s", supplyBefore, supplyAfter)
		}
		_ = transparency.ValidateNoCleartextPII(metadata)
	})
}

