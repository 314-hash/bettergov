// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title PublicTransparencyRegistry
 * @notice BETTERGOV On-Chain Public Transparency & Accountability Registry.
 * @dev Stores cryptographic SHA-256 hashes and verification metadata only.
 *      Sensitive Personally Identifiable Information (PII) MUST remain off-chain.
 *      Recording a hash proves timestamp and data integrity only; it does NOT imply
 *      authenticity of the underlying claim unless status == VerificationStatus.AuditorVerified.
 */
contract PublicTransparencyRegistry {
    string public constant AUTHENTICITY_DISCLAIMER =
        "ON-CHAIN RECORD PROVES CRYPTOGRAPHIC INTEGRITY AND TIMESTAMP ONLY. DOES NOT IMPLY GOVERNMENT OR AUDITOR ENDORSEMENT UNLESS STATUS IS AUDITOR_VERIFIED.";

    enum VerificationStatus {
        UnverifiedHashAnchor,
        AuditorVerified,
        Disputed,
        Revoked
    }

    struct DocumentAnchor {
        bytes32 id;
        bytes32 documentSha256;
        string agencyCode;
        string offChainUri;
        address submitter;
        uint64 recordedAtBlock;
        uint64 timestamp;
        VerificationStatus status;
        address verifiedByAuditor;
        bytes32 auditorAttestationHash;
    }

    struct ProcurementRecord {
        bytes32 id;
        string tenderReference;
        string procuringEntity;
        bytes32 tenderNoticeHash;
        bytes32 bidEvaluationHash;
        address winningBidder;
        uint256 contractValueAbgov;
        uint32 bidsCount;
        VerificationStatus status;
        uint64 recordedAt;
    }

    struct GovernmentProject {
        bytes32 id;
        string projectName;
        string department;
        bytes32 specificationHash;
        address contractor;
        uint256 totalBudgetAbgov;
        uint256 disbursedAbgov;
        uint16 completionBps;
        VerificationStatus status;
    }

    address public governanceAuthority;
    mapping(address => bool) public authorizedAuditors;
    mapping(bytes32 => DocumentAnchor) public documents;
    mapping(bytes32 => ProcurementRecord) public procurements;
    mapping(bytes32 => GovernmentProject) public projects;

    bytes32[] public documentIds;
    bytes32[] public procurementIds;
    bytes32[] public projectIds;

    event AuditorUpdated(address indexed auditor, bool authorized);
    event DocumentAnchored(
        bytes32 indexed id,
        bytes32 indexed documentSha256,
        string agencyCode,
        address indexed submitter,
        VerificationStatus initialStatus,
        string disclaimer
    );
    event DocumentAuditVerified(
        bytes32 indexed id,
        address indexed auditor,
        VerificationStatus verdict,
        bytes32 auditorAttestationHash
    );
    event ProcurementRecorded(
        bytes32 indexed id,
        string tenderReference,
        address indexed winningBidder,
        uint256 contractValueAbgov
    );
    event ProjectRegistered(
        bytes32 indexed id,
        string projectName,
        address indexed contractor,
        uint256 totalBudgetAbgov
    );

    error UnauthorizedGovernance();
    error UnauthorizedAuditor();
    error RecordAlreadyExists();
    error RecordNotFound();
    error InvalidHash();

    modifier onlyGovernance() {
        if (msg.sender != governanceAuthority) revert UnauthorizedGovernance();
        _;
    }

    modifier onlyAuditor() {
        if (!authorizedAuditors[msg.sender]) revert UnauthorizedAuditor();
        _;
    }

    constructor(address _governanceAuthority, address[] memory _initialAuditors) {
        governanceAuthority = _governanceAuthority == address(0) ? msg.sender : _governanceAuthority;
        for (uint256 i = 0; i < _initialAuditors.length; i++) {
            authorizedAuditors[_initialAuditors[i]] = true;
            emit AuditorUpdated(_initialAuditors[i], true);
        }
    }

    function setAuditor(address auditor, bool authorized) external onlyGovernance {
        authorizedAuditors[auditor] = authorized;
        emit AuditorUpdated(auditor, authorized);
    }

    function anchorDocument(
        bytes32 id,
        bytes32 documentSha256,
        string calldata agencyCode,
        string calldata offChainUri
    ) external {
        if (documentSha256 == bytes32(0)) revert InvalidHash();
        if (documents[id].recordedAtBlock != 0) revert RecordAlreadyExists();

        documents[id] = DocumentAnchor({
            id: id,
            documentSha256: documentSha256,
            agencyCode: agencyCode,
            offChainUri: offChainUri,
            submitter: msg.sender,
            recordedAtBlock: uint64(block.number),
            timestamp: uint64(block.timestamp),
            status: VerificationStatus.UnverifiedHashAnchor,
            verifiedByAuditor: address(0),
            auditorAttestationHash: bytes32(0)
        });
        documentIds.push(id);

        emit DocumentAnchored(
            id,
            documentSha256,
            agencyCode,
            msg.sender,
            VerificationStatus.UnverifiedHashAnchor,
            AUTHENTICITY_DISCLAIMER
        );
    }

    function verifyDocumentByAuditor(
        bytes32 id,
        VerificationStatus verdict,
        bytes32 auditorAttestationHash
    ) external onlyAuditor {
        DocumentAnchor storage doc = documents[id];
        if (doc.recordedAtBlock == 0) revert RecordNotFound();
        if (auditorAttestationHash == bytes32(0)) revert InvalidHash();

        doc.status = verdict;
        doc.verifiedByAuditor = msg.sender;
        doc.auditorAttestationHash = auditorAttestationHash;

        emit DocumentAuditVerified(id, msg.sender, verdict, auditorAttestationHash);
    }

    function recordProcurement(
        bytes32 id,
        string calldata tenderReference,
        string calldata procuringEntity,
        bytes32 tenderNoticeHash,
        bytes32 bidEvaluationHash,
        address winningBidder,
        uint256 contractValueAbgov,
        uint32 bidsCount
    ) external {
        if (tenderNoticeHash == bytes32(0) || bidEvaluationHash == bytes32(0)) revert InvalidHash();
        if (procurements[id].recordedAt != 0) revert RecordAlreadyExists();

        procurements[id] = ProcurementRecord({
            id: id,
            tenderReference: tenderReference,
            procuringEntity: procuringEntity,
            tenderNoticeHash: tenderNoticeHash,
            bidEvaluationHash: bidEvaluationHash,
            winningBidder: winningBidder,
            contractValueAbgov: contractValueAbgov,
            bidsCount: bidsCount,
            status: VerificationStatus.UnverifiedHashAnchor,
            recordedAt: uint64(block.timestamp)
        });
        procurementIds.push(id);

        emit ProcurementRecorded(id, tenderReference, winningBidder, contractValueAbgov);
    }

    function registerProject(
        bytes32 id,
        string calldata projectName,
        string calldata department,
        bytes32 specificationHash,
        address contractor,
        uint256 totalBudgetAbgov
    ) external {
        if (specificationHash == bytes32(0)) revert InvalidHash();
        if (projects[id].contractor != address(0)) revert RecordAlreadyExists();

        projects[id] = GovernmentProject({
            id: id,
            projectName: projectName,
            department: department,
            specificationHash: specificationHash,
            contractor: contractor,
            totalBudgetAbgov: totalBudgetAbgov,
            disbursedAbgov: 0,
            completionBps: 0,
            status: VerificationStatus.UnverifiedHashAnchor
        });
        projectIds.push(id);

        emit ProjectRegistered(id, projectName, contractor, totalBudgetAbgov);
    }

    function getCounts() external view returns (uint256 docs, uint256 procs, uint256 projs) {
        return (documentIds.length, procurementIds.length, projectIds.length);
    }
}

