// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title CivicSBT
 * @notice Non-Transferable Soulbound Token (SBT) Credentials for Academic Degrees,
 *         Civic Identities, and Public Reputation on BETTERGOV Chain.
 * @dev Eliminates speculative secondary markets by permanently locking all issued
 *      credentials (ERC-5192 compatible `locked(tokenId) == true`) and reverting
 *      on any transfer, transferFrom, or approval call.
 *      Stores only cryptographic commitment hashes on-chain (zero cleartext PII).
 */
contract CivicSBT {
    enum CredentialCategory {
        ACADEMIC_DEGREE_LEVEL,
        CIVIC_IDENTITY_TIER,
        PUBLIC_AUDITOR_REPUTATION,
        CLEAN_ENERGY_PRODUCER_BADGE,
        AGRICULTURAL_PRODUCER_BADGE
    }

    struct SoulboundCredential {
        uint256 tokenId;
        address holder;
        CredentialCategory category;
        uint16 levelOrTier;
        bytes32 credentialCommitmentHash; // Salted SHA-256 hash; never raw PII
        string issuerAgencyCode;
        uint64 issuedAt;
        bool revoked;
    }

    string public constant name = "BETTERGOV Civic Soulbound Credential";
    string public constant symbol = "BGSBT";

    address public governanceAuthority;
    uint256 public nextTokenId = 1;

    mapping(address => bool) public authorizedIssuers;
    mapping(uint256 => SoulboundCredential) public credentials;
    mapping(address => uint256[]) public holderTokenIds;

    // ERC-5192 Soulbound Lock Event
    event Locked(uint256 indexed tokenId);
    event CredentialIssued(
        uint256 indexed tokenId,
        address indexed holder,
        CredentialCategory indexed category,
        uint16 levelOrTier,
        string issuerAgencyCode,
        bytes32 credentialCommitmentHash
    );
    event CredentialRevoked(uint256 indexed tokenId, address indexed revokedBy, string reason);

    error OnlyGovernance();
    error OnlyAuthorizedIssuer();
    error SoulboundNonTransferable();
    error InvalidCredentialParameters();
    error CredentialNotFound();

    modifier onlyGovernance() {
        if (msg.sender != governanceAuthority) revert OnlyGovernance();
        _;
    }

    modifier onlyIssuer() {
        if (!authorizedIssuers[msg.sender] && msg.sender != governanceAuthority) {
            revert OnlyAuthorizedIssuer();
        }
        _;
    }

    constructor(address _governanceAuthority) {
        governanceAuthority = _governanceAuthority == address(0) ? msg.sender : _governanceAuthority;
        authorizedIssuers[governanceAuthority] = true;
    }

    function setAuthorizedIssuer(address issuer, bool authorized) external onlyGovernance {
        authorizedIssuers[issuer] = authorized;
    }

    /**
     * @notice Issues a permanently soulbound (non-transferable) credential.
     */
    function issueSoulboundCredential(
        address holder,
        CredentialCategory category,
        uint16 levelOrTier,
        bytes32 credentialCommitmentHash,
        string calldata issuerAgencyCode
    ) external onlyIssuer returns (uint256 tokenId) {
        if (holder == address(0) || credentialCommitmentHash == bytes32(0) || levelOrTier == 0) {
            revert InvalidCredentialParameters();
        }

        tokenId = nextTokenId++;
        credentials[tokenId] = SoulboundCredential({
            tokenId: tokenId,
            holder: holder,
            category: category,
            levelOrTier: levelOrTier,
            credentialCommitmentHash: credentialCommitmentHash,
            issuerAgencyCode: issuerAgencyCode,
            issuedAt: uint64(block.timestamp),
            revoked: false
        });
        holderTokenIds[holder].push(tokenId);

        emit Locked(tokenId);
        emit CredentialIssued(
            tokenId,
            holder,
            category,
            levelOrTier,
            issuerAgencyCode,
            credentialCommitmentHash
        );
    }

    function revokeCredential(uint256 tokenId, string calldata reason) external onlyIssuer {
        SoulboundCredential storage cred = credentials[tokenId];
        if (cred.holder == address(0)) revert CredentialNotFound();
        cred.revoked = true;
        emit CredentialRevoked(tokenId, msg.sender, reason);
    }

    /**
     * @notice ERC-5192 standard check: always returns true because all credentials are soulbound.
     */
    function locked(uint256 tokenId) external view returns (bool) {
        if (credentials[tokenId].holder == address(0)) revert CredentialNotFound();
        return true;
    }

    /**
     * @notice Transfers and approvals are unconditionally prohibited to prevent secondary market speculation.
     */
    function transferFrom(address, address, uint256) external pure {
        revert SoulboundNonTransferable();
    }

    function safeTransferFrom(address, address, uint256) external pure {
        revert SoulboundNonTransferable();
    }

    function approve(address, uint256) external pure {
        revert SoulboundNonTransferable();
    }

    function setApprovalForAll(address, bool) external pure {
        revert SoulboundNonTransferable();
    }
}

