package transparency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const MandatoryAuthenticityDisclaimer = "ON-CHAIN RECORD PROVES CRYPTOGRAPHIC INTEGRITY AND TIMESTAMP ONLY. RECORDING A HASH ON BETTERGOV DOES NOT AUTOMATICALLY VALIDATE THE UNDERLYING DOCUMENT OR GOVERNMENT CLAIM UNLESS INDEPENDENTLY VERIFIED BY AUTHORIZED AUDITORS."

type VerificationStatus string

const (
	StatusUnverifiedHashAnchor VerificationStatus = "UNVERIFIED_HASH_ANCHOR"
	StatusAuditorVerified      VerificationStatus = "AUDITOR_VERIFIED"
	StatusDisputed             VerificationStatus = "DISPUTED"
	StatusRevoked              VerificationStatus = "REVOKED"
)

var (
	ErrCleartextPIIDetected = errors.New("rejected: sensitive personal information (PII) detected in on-chain metadata; store PII off-chain and submit cryptographic hashes only")
	ErrInvalidHashFormat    = errors.New("cryptographic hash must be a valid 64-character hex SHA-256 digest")
	ErrRecordNotFound       = errors.New("transparency record not found")
	ErrUnauthorizedAuditor  = errors.New("caller is not a registered independent public auditor")
)

var (
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	ssnRegex   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	phoneRegex = regexp.MustCompile(`\b(?:\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]\d{3}[-.\s]\d{4}\b`)
	sha256Hex  = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
)

// ValidateNoCleartextPII ensures sensitive personal information is never stored on-chain.
func ValidateNoCleartextPII(fields ...string) error {
	for _, f := range fields {
		if emailRegex.MatchString(f) || ssnRegex.MatchString(f) || phoneRegex.MatchString(f) {
			return ErrCleartextPIIDetected
		}
	}
	return nil
}

func ValidateSHA256Hex(h string) error {
	h = strings.TrimPrefix(strings.TrimSpace(h), "0x")
	if !sha256Hex.MatchString(h) {
		return ErrInvalidHashFormat
	}
	return nil
}

// 1. Public Document Verification Record
type DocumentRecord struct {
	ID                     string             `json:"id"`
	Title                  string             `json:"title"`
	AgencyCode             string             `json:"agency_code"`
	DocumentSHA256         string             `json:"document_sha256"`
	OffChainURI            string             `json:"off_chain_uri"`
	Submitter              string             `json:"submitter"`
	BlockHeight            int64              `json:"block_height"`
	Timestamp              time.Time          `json:"timestamp"`
	VerificationStatus     VerificationStatus `json:"verification_status"`
	VerifiedByAuditor      string             `json:"verified_by_auditor,omitempty"`
	AuditorAttestationHash string             `json:"auditor_attestation_hash,omitempty"`
	AttestationDisclaimer  string             `json:"attestation_disclaimer"`
}

// 2. Government Project Tracking Record
type GovernmentProject struct {
	ID                    string             `json:"id"`
	ProjectName           string             `json:"project_name"`
	Department            string             `json:"department"`
	Region                string             `json:"region"`
	TotalBudgetAbgov      *big.Int           `json:"total_budget_abgov"`
	DisbursedAbgov        *big.Int           `json:"disbursed_abgov"`
	CompletionBps         uint64             `json:"completion_bps"` // 0 - 10000 (100.00%)
	SpecificationHash     string             `json:"specification_hash"`
	ContractorAddress     string             `json:"contractor_address"`
	MilestonesCompleted   int                `json:"milestones_completed"`
	TotalMilestones       int                `json:"total_milestones"`
	VerificationStatus    VerificationStatus `json:"verification_status"`
	AttestationDisclaimer string             `json:"attestation_disclaimer"`
	UpdatedAt             time.Time          `json:"updated_at"`
}

// 3. Procurement Transparency Record
type ProcurementRecord struct {
	ID                    string             `json:"id"`
	TenderReference       string             `json:"tender_reference"`
	ProcuringEntity       string             `json:"procuring_entity"`
	TenderNoticeHash      string             `json:"tender_notice_hash"`
	BidEvaluationHash     string             `json:"bid_evaluation_hash"`
	WinningBidderAddress  string             `json:"winning_bidder_address"`
	ContractValueAbgov    *big.Int           `json:"contract_value_abgov"`
	BidsCount             int                `json:"bids_count"`
	VerificationStatus    VerificationStatus `json:"verification_status"`
	AttestationDisclaimer string             `json:"attestation_disclaimer"`
	RecordedAt            time.Time          `json:"recorded_at"`
}

// 4. Budget Allocation Record
type BudgetAllocation struct {
	ID                    string             `json:"id"`
	FiscalYear            int                `json:"fiscal_year"`
	Department            string             `json:"department"`
	ProgramCode           string             `json:"program_code"`
	AllocatedAbgov        *big.Int           `json:"allocated_abgov"`
	ReleasedAbgov         *big.Int           `json:"released_abgov"`
	AppropriationLawHash  string             `json:"appropriation_law_hash"`
	VerificationStatus    VerificationStatus `json:"verification_status"`
	AttestationDisclaimer string             `json:"attestation_disclaimer"`
	Timestamp             time.Time          `json:"timestamp"`
}

// 5. Community Participation Record
type CommunityParticipation struct {
	ID                    string    `json:"id"`
	TopicID               string    `json:"topic_id"`
	CitizenPseudonymAddr  string    `json:"citizen_pseudonym_addr"`
	SubmissionSHA256      string    `json:"submission_sha256"`
	Category              string    `json:"category"`
	BlockHeight           int64     `json:"block_height"`
	Timestamp             time.Time `json:"timestamp"`
	AttestationDisclaimer string    `json:"attestation_disclaimer"`
}

// 6. Public Audit Evidence Record
type AuditEvidenceRecord struct {
	ID                    string             `json:"id"`
	TargetRecordID        string             `json:"target_record_id"`
	AuditorAddress        string             `json:"auditor_address"`
	EvidenceSHA256        string             `json:"evidence_sha256"`
	MethodologySummary    string             `json:"methodology_summary"`
	Verdict               VerificationStatus `json:"verdict"`
	Timestamp             time.Time          `json:"timestamp"`
	AttestationDisclaimer string             `json:"attestation_disclaimer"`
}

// 7. Digital Certificate Record
type DigitalCertificate struct {
	ID                    string    `json:"id"`
	CertificateSHA256     string    `json:"certificate_sha256"`
	IssuerAgency          string    `json:"issuer_agency"`
	SubjectCommitmentHash string    `json:"subject_commitment_hash"` // Salted hash of recipient; never raw PII
	IssuedAt              time.Time `json:"issued_at"`
	ExpiresAt             time.Time `json:"expires_at"`
	Revoked               bool      `json:"revoked"`
	RevocationReason      string    `json:"revocation_reason,omitempty"`
	AttestationDisclaimer string    `json:"attestation_disclaimer"`
}

// 8. Timestamped Anchor Record
type TimestampRecord struct {
	ID                    string    `json:"id"`
	PayloadSHA256         string    `json:"payload_sha256"`
	Submitter             string    `json:"submitter"`
	BlockHeight           int64     `json:"block_height"`
	BlockTimestamp        time.Time `json:"block_timestamp"`
	MerkleAnchor          string    `json:"merkle_anchor"`
	AttestationDisclaimer string    `json:"attestation_disclaimer"`
}

type Keeper struct {
	mu                 sync.RWMutex
	registeredAuditors map[string]bool
	documents          map[string]*DocumentRecord
	projects           map[string]*GovernmentProject
	procurements       map[string]*ProcurementRecord
	budgets            map[string]*BudgetAllocation
	participations     map[string]*CommunityParticipation
	auditEvidences     map[string]*AuditEvidenceRecord
	certificates       map[string]*DigitalCertificate
	timestamps         map[string]*TimestampRecord
}

func NewKeeper() *Keeper {
	return &Keeper{
		registeredAuditors: make(map[string]bool),
		documents:          make(map[string]*DocumentRecord),
		projects:           make(map[string]*GovernmentProject),
		procurements:       make(map[string]*ProcurementRecord),
		budgets:            make(map[string]*BudgetAllocation),
		participations:     make(map[string]*CommunityParticipation),
		auditEvidences:     make(map[string]*AuditEvidenceRecord),
		certificates:       make(map[string]*DigitalCertificate),
		timestamps:         make(map[string]*TimestampRecord),
	}
}

func (k *Keeper) RegisterAuditor(addr string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.registeredAuditors[strings.ToLower(strings.TrimSpace(addr))] = true
}

func (k *Keeper) IsAuditor(addr string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.registeredAuditors[strings.ToLower(strings.TrimSpace(addr))]
}

// RecordDocument anchors a public document's cryptographic hash on-chain after enforcing the PII guard.
func (k *Keeper) RecordDocument(id, title, agencyCode, docSHA256, offChainURI, submitter string, height int64, blockTime time.Time) (*DocumentRecord, error) {
	if err := ValidateNoCleartextPII(title, agencyCode, offChainURI); err != nil {
		return nil, err
	}
	if err := ValidateSHA256Hex(docSHA256); err != nil {
		return nil, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	rec := &DocumentRecord{
		ID:                    id,
		Title:                 title,
		AgencyCode:            agencyCode,
		DocumentSHA256:        strings.ToLower(strings.TrimPrefix(docSHA256, "0x")),
		OffChainURI:           offChainURI,
		Submitter:             strings.ToLower(submitter),
		BlockHeight:           height,
		Timestamp:             blockTime,
		VerificationStatus:    StatusUnverifiedHashAnchor,
		AttestationDisclaimer: MandatoryAuthenticityDisclaimer,
	}
	k.documents[id] = rec
	return rec, nil
}

// VerifyDocumentByAuditor allows an authorized independent auditor to attach a signed audit evidence hash and update status.
func (k *Keeper) VerifyDocumentByAuditor(docID, auditorAddr, attestationHash, methodology string, verdict VerificationStatus, blockTime time.Time) (*DocumentRecord, error) {
	auditorAddr = strings.ToLower(strings.TrimSpace(auditorAddr))
	if err := ValidateSHA256Hex(attestationHash); err != nil {
		return nil, err
	}
	if err := ValidateNoCleartextPII(methodology); err != nil {
		return nil, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.registeredAuditors[auditorAddr] {
		return nil, ErrUnauthorizedAuditor
	}
	doc, ok := k.documents[docID]
	if !ok {
		return nil, ErrRecordNotFound
	}
	doc.VerificationStatus = verdict
	doc.VerifiedByAuditor = auditorAddr
	doc.AuditorAttestationHash = strings.ToLower(strings.TrimPrefix(attestationHash, "0x"))

	evID := fmt.Sprintf("audit-%s-%d", docID, len(k.auditEvidences)+1)
	k.auditEvidences[evID] = &AuditEvidenceRecord{
		ID:                    evID,
		TargetRecordID:        docID,
		AuditorAddress:        auditorAddr,
		EvidenceSHA256:        doc.AuditorAttestationHash,
		MethodologySummary:    methodology,
		Verdict:               verdict,
		Timestamp:             blockTime,
		AttestationDisclaimer: MandatoryAuthenticityDisclaimer,
	}
	return doc, nil
}

func (k *Keeper) RegisterProject(
	id, name, dept, region, specHash, contractor string,
	budgetAbgov *big.Int,
	totalMilestones int,
	blockTime time.Time,
) (*GovernmentProject, error) {
	if err := ValidateNoCleartextPII(name, dept, region); err != nil {
		return nil, err
	}
	if err := ValidateSHA256Hex(specHash); err != nil {
		return nil, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	proj := &GovernmentProject{
		ID:                    id,
		ProjectName:           name,
		Department:            dept,
		Region:                region,
		TotalBudgetAbgov:      new(big.Int).Set(budgetAbgov),
		DisbursedAbgov:        big.NewInt(0),
		CompletionBps:         0,
		SpecificationHash:     strings.ToLower(strings.TrimPrefix(specHash, "0x")),
		ContractorAddress:     strings.ToLower(contractor),
		MilestonesCompleted:   0,
		TotalMilestones:       totalMilestones,
		VerificationStatus:    StatusUnverifiedHashAnchor,
		AttestationDisclaimer: MandatoryAuthenticityDisclaimer,
		UpdatedAt:             blockTime,
	}
	k.projects[id] = proj
	return proj, nil
}

func (k *Keeper) RecordProcurement(
	id, tenderRef, entity, noticeHash, evalHash, winnerAddr string,
	contractValue *big.Int,
	bidsCount int,
	blockTime time.Time,
) (*ProcurementRecord, error) {
	if err := ValidateNoCleartextPII(tenderRef, entity); err != nil {
		return nil, err
	}
	if err := ValidateSHA256Hex(noticeHash); err != nil {
		return nil, err
	}
	if err := ValidateSHA256Hex(evalHash); err != nil {
		return nil, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	rec := &ProcurementRecord{
		ID:                    id,
		TenderReference:       tenderRef,
		ProcuringEntity:       entity,
		TenderNoticeHash:      strings.ToLower(strings.TrimPrefix(noticeHash, "0x")),
		BidEvaluationHash:     strings.ToLower(strings.TrimPrefix(evalHash, "0x")),
		WinningBidderAddress:  strings.ToLower(winnerAddr),
		ContractValueAbgov:    new(big.Int).Set(contractValue),
		BidsCount:             bidsCount,
		VerificationStatus:    StatusUnverifiedHashAnchor,
		AttestationDisclaimer: MandatoryAuthenticityDisclaimer,
		RecordedAt:            blockTime,
	}
	k.procurements[id] = rec
	return rec, nil
}

func (k *Keeper) RecordBudgetAllocation(
	id string,
	fiscalYear int,
	department, programCode, lawHash string,
	allocated, released *big.Int,
	blockTime time.Time,
) (*BudgetAllocation, error) {
	if err := ValidateNoCleartextPII(department, programCode); err != nil {
		return nil, err
	}
	if err := ValidateSHA256Hex(lawHash); err != nil {
		return nil, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	rec := &BudgetAllocation{
		ID:                    id,
		FiscalYear:            fiscalYear,
		Department:            department,
		ProgramCode:           programCode,
		AllocatedAbgov:        new(big.Int).Set(allocated),
		ReleasedAbgov:         new(big.Int).Set(released),
		AppropriationLawHash:  strings.ToLower(strings.TrimPrefix(lawHash, "0x")),
		VerificationStatus:    StatusUnverifiedHashAnchor,
		AttestationDisclaimer: MandatoryAuthenticityDisclaimer,
		Timestamp:             blockTime,
	}
	k.budgets[id] = rec
	return rec, nil
}

func (k *Keeper) RecordCommunityParticipation(id, topicID, citizenAddr, submissionHash, category string, height int64, blockTime time.Time) (*CommunityParticipation, error) {
	if err := ValidateNoCleartextPII(topicID, category); err != nil {
		return nil, err
	}
	if err := ValidateSHA256Hex(submissionHash); err != nil {
		return nil, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	rec := &CommunityParticipation{
		ID:                    id,
		TopicID:               topicID,
		CitizenPseudonymAddr:  strings.ToLower(citizenAddr),
		SubmissionSHA256:      strings.ToLower(strings.TrimPrefix(submissionHash, "0x")),
		Category:              category,
		BlockHeight:           height,
		Timestamp:             blockTime,
		AttestationDisclaimer: MandatoryAuthenticityDisclaimer,
	}
	k.participations[id] = rec
	return rec, nil
}

func (k *Keeper) IssueDigitalCertificate(id, certHash, issuerAgency, subjectCommitmentHash string, issuedAt, expiresAt time.Time) (*DigitalCertificate, error) {
	if err := ValidateNoCleartextPII(issuerAgency); err != nil {
		return nil, err
	}
	if err := ValidateSHA256Hex(certHash); err != nil {
		return nil, err
	}
	if err := ValidateSHA256Hex(subjectCommitmentHash); err != nil {
		return nil, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	cert := &DigitalCertificate{
		ID:                    id,
		CertificateSHA256:     strings.ToLower(strings.TrimPrefix(certHash, "0x")),
		IssuerAgency:          issuerAgency,
		SubjectCommitmentHash: strings.ToLower(strings.TrimPrefix(subjectCommitmentHash, "0x")),
		IssuedAt:              issuedAt,
		ExpiresAt:             expiresAt,
		Revoked:               false,
		AttestationDisclaimer: MandatoryAuthenticityDisclaimer,
	}
	k.certificates[id] = cert
	return cert, nil
}

func (k *Keeper) AnchorTimestamp(id, payloadHash, submitter string, height int64, blockTime time.Time) (*TimestampRecord, error) {
	if err := ValidateSHA256Hex(payloadHash); err != nil {
		return nil, err
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	anchorPreimage := fmt.Sprintf("%s:%s:%d:%d", id, payloadHash, height, blockTime.UnixNano())
	sum := sha256.Sum256([]byte(anchorPreimage))

	rec := &TimestampRecord{
		ID:                    id,
		PayloadSHA256:         strings.ToLower(strings.TrimPrefix(payloadHash, "0x")),
		Submitter:             strings.ToLower(submitter),
		BlockHeight:           height,
		BlockTimestamp:        blockTime,
		MerkleAnchor:          hex.EncodeToString(sum[:]),
		AttestationDisclaimer: MandatoryAuthenticityDisclaimer,
	}
	k.timestamps[id] = rec
	return rec, nil
}

func (k *Keeper) GetAllDocuments() []*DocumentRecord {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]*DocumentRecord, 0, len(k.documents))
	for _, d := range k.documents {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (k *Keeper) GetAllProjects() []*GovernmentProject {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]*GovernmentProject, 0, len(k.projects))
	for _, p := range k.projects {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (k *Keeper) GetAllProcurements() []*ProcurementRecord {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]*ProcurementRecord, 0, len(k.procurements))
	for _, p := range k.procurements {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (k *Keeper) GetAllBudgets() []*BudgetAllocation {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]*BudgetAllocation, 0, len(k.budgets))
	for _, b := range k.budgets {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (k *Keeper) StateRoot() string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	payload, _ := json.Marshal(struct {
		Docs         int `json:"docs"`
		Projects     int `json:"projects"`
		Procurements int `json:"procurements"`
		Budgets      int `json:"budgets"`
		Certs        int `json:"certs"`
		Timestamps   int `json:"timestamps"`
	}{
		Docs:         len(k.documents),
		Projects:     len(k.projects),
		Procurements: len(k.procurements),
		Budgets:      len(k.budgets),
		Certs:        len(k.certificates),
		Timestamps:   len(k.timestamps),
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

