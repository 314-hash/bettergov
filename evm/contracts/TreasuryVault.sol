// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title TreasuryVault
 * @notice Governance-controlled public treasury vault with Activity-Based Milestone Releases,
 *         BSP Circular No. 1108 VASP Separation (zero direct PHP fiat rails), and Reentrancy protection.
 */
contract TreasuryVault {
    string public constant BSP_1108_VASP_SEPARATION =
        "CRYPTO_UTILITY_LAYER_ONLY: ZERO_DIRECT_PHP_FIAT_RAILS_CONVERSIONS_OFFCHAIN_VIA_LICENSED_VASP";

    enum ActiveContributionCategory {
        PUBLIC_INFRASTRUCTURE_DELIVERY,
        CLEAN_ENERGY_GENERATION,
        AGRICULTURAL_CROP_SUBMISSION,
        GOVERNANCE_AND_AUDIT_WORK
    }

    struct AllocationGrant {
        bytes32 grantId;
        address recipient;
        uint256 totalAllocated;
        uint256 totalReleased;
        bytes32 auditProofHash;
        bytes32 activityContributionProofHash;
        ActiveContributionCategory activityCategory;
        bool active;
    }

    address public governanceExecutor;
    address public emergencyGuardian;
    bool public paused;
    uint256 private unlocked = 1;

    mapping(bytes32 => AllocationGrant) public grants;

    event TreasuryFunded(address indexed sender, uint256 amount);
    event GrantCreated(bytes32 indexed grantId, address indexed recipient, uint256 totalAllocated);
    event MilestoneReleased(
        bytes32 indexed grantId,
        address indexed recipient,
        uint256 amount,
        ActiveContributionCategory indexed activityCategory,
        bytes32 auditProofHash,
        bytes32 activityContributionProofHash
    );
    event EmergencyPauseToggled(bool paused, address indexed triggeredBy);

    error OnlyGovernance();
    error OnlyGuardianOrGovernance();
    error VaultPaused();
    error ReentrancyBlocked();
    error InvalidGrant();
    error PassiveReleaseProhibitedRequiresActivityProof();
    error ExceedsAllocation();
    error TransferFailed();

    modifier nonReentrant() {
        if (unlocked != 1) revert ReentrancyBlocked();
        unlocked = 2;
        _;
        unlocked = 1;
    }

    modifier onlyGovernance() {
        if (msg.sender != governanceExecutor) revert OnlyGovernance();
        _;
    }

    modifier whenNotPaused() {
        if (paused) revert VaultPaused();
        _;
    }

    constructor(address _governanceExecutor, address _emergencyGuardian) {
        governanceExecutor = _governanceExecutor == address(0) ? msg.sender : _governanceExecutor;
        emergencyGuardian = _emergencyGuardian == address(0) ? msg.sender : _emergencyGuardian;
    }

    receive() external payable {
        emit TreasuryFunded(msg.sender, msg.value);
    }

    function setPaused(bool _paused) external {
        if (msg.sender != emergencyGuardian && msg.sender != governanceExecutor) {
            revert OnlyGuardianOrGovernance();
        }
        paused = _paused;
        emit EmergencyPauseToggled(_paused, msg.sender);
    }

    function createGrant(
        bytes32 grantId,
        address recipient,
        uint256 totalAllocated
    ) external onlyGovernance whenNotPaused {
        if (recipient == address(0) || totalAllocated == 0 || grants[grantId].active) {
            revert InvalidGrant();
        }
        grants[grantId] = AllocationGrant({
            grantId: grantId,
            recipient: recipient,
            totalAllocated: totalAllocated,
            totalReleased: 0,
            auditProofHash: bytes32(0),
            activityContributionProofHash: bytes32(0),
            activityCategory: ActiveContributionCategory.PUBLIC_INFRASTRUCTURE_DELIVERY,
            active: true
        });
        emit GrantCreated(grantId, recipient, totalAllocated);
    }

    /**
     * @notice Releases a treasury milestone strictly tied to an active contribution proof
     *         (SEC Classification Mitigation: no passive return distributions).
     */
    function releaseMilestone(
        bytes32 grantId,
        uint256 amount,
        ActiveContributionCategory activityCategory,
        bytes32 auditProofHash,
        bytes32 activityContributionProofHash
    ) external onlyGovernance whenNotPaused nonReentrant {
        AllocationGrant storage g = grants[grantId];
        if (!g.active || auditProofHash == bytes32(0) || amount == 0) revert InvalidGrant();
        if (activityContributionProofHash == bytes32(0)) {
            revert PassiveReleaseProhibitedRequiresActivityProof();
        }
        if (g.totalReleased + amount > g.totalAllocated) revert ExceedsAllocation();
        if (address(this).balance < amount) revert ExceedsAllocation();

        g.totalReleased += amount;
        g.auditProofHash = auditProofHash;
        g.activityContributionProofHash = activityContributionProofHash;
        g.activityCategory = activityCategory;

        (bool ok, ) = g.recipient.call{value: amount}("");
        if (!ok) revert TransferFailed();

        emit MilestoneReleased(
            grantId,
            g.recipient,
            amount,
            activityCategory,
            auditProofHash,
            activityContributionProofHash
        );
    }
}
