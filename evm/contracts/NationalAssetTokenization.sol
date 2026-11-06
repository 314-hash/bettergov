// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title NationalAssetTokenization
 * @notice SEC Classification Mitigation & BSP Circular No. 1108 Compliant RWA Utility Registry.
 * @dev Implements three strict regulatory & architectural safeguards:
 *      1. Non-Dividend RWA Shares: Fractional RWA units represent functional access to infrastructure
 *         and distribute outputs EXCLUSIVELY as Functional Utility Discounts/Credits (e.g., warehouse
 *         storage cubic-meter credits or solar energy kWh discounts). Monetary/fiat dividend payouts
 *         are disallowed at the protocol level to mitigate SEC investment-contract classification.
 *      2. Activity-Based Rewards: Utility credit unlocks require verifiable active contributions
 *         (generating clean energy, submitting verified agricultural crops, or governance voting)
 *         rather than passive capital holding.
 *      3. BSP Circular No. 1108 (VASP Separation): Operates strictly at the crypto-utility layer.
 *         Zero direct fiat (PHP) rails or remittance functions exist in this contract.
 */
contract NationalAssetTokenization {
    string public constant REGULATORY_POSTURE =
        "SEC_MITIGATION: NON_DIVIDEND_UTILITY_CREDITS_ONLY | BSP_1108: CRYPTO_UTILITY_LAYER_ZERO_FIAT_RAILS";

    enum UtilityCreditType {
        SOLAR_ENERGY_KWH_DISCOUNT,
        WAREHOUSE_STORAGE_CREDIT,
        IRRIGATION_WATER_ALLOCATION,
        COLD_CHAIN_LOGISTICS_OFFSET
    }

    enum ActivityContributionType {
        CLEAN_ENERGY_GENERATION,
        CROP_HARVEST_SUBMISSION,
        GOVERNANCE_PROPOSAL_VOTE,
        PUBLIC_AUDIT_ATTESTATION
    }

    struct NationalAsset {
        bytes32 assetId;
        string assetName;
        string locationRegion;
        UtilityCreditType creditType;
        uint256 totalCapacityUnits;
        uint256 allocatedUnits;
        bool active;
    }

    struct ActivityRecord {
        uint256 cleanEnergyKwhGenerated;
        uint256 cropBatchesSubmitted;
        uint256 governanceVotesCast;
        uint256 totalActivityPoints;
        bytes32 lastActivityProofHash;
        uint64 lastUpdatedBlock;
    }

    address public governanceAuthority;
    mapping(address => bool) public authorizedVerifiers;
    mapping(bytes32 => NationalAsset) public assets;
    bytes32[] public assetIds;

    // assetId => participant => fractional utility units held
    mapping(bytes32 => mapping(address => uint256)) public utilityUnits;
    // assetId => participant => functional utility discount credits earned (in kWh / cubic-meter units, NEVER fiat/monetary dividends)
    mapping(bytes32 => mapping(address => uint256)) public functionalUtilityCredits;
    // participant => active contribution record
    mapping(address => ActivityRecord) public participantActivity;

    event AssetRegistered(
        bytes32 indexed assetId,
        string assetName,
        UtilityCreditType creditType,
        uint256 totalCapacityUnits
    );
    event ActivityContributionVerified(
        address indexed participant,
        ActivityContributionType indexed activityType,
        uint256 quantity,
        uint256 newActivityPoints,
        bytes32 proofHash
    );
    event UtilityUnitsAllocated(bytes32 indexed assetId, address indexed participant, uint256 units);
    event FunctionalUtilityCreditIssued(
        bytes32 indexed assetId,
        address indexed participant,
        UtilityCreditType creditType,
        uint256 utilityDiscountUnits,
        uint256 activityPointsApplied
    );
    event FunctionalUtilityCreditRedeemed(
        bytes32 indexed assetId,
        address indexed participant,
        UtilityCreditType creditType,
        uint256 redeemedUnits,
        bytes32 serviceReceiptHash
    );

    error OnlyGovernance();
    error OnlyAuthorizedVerifier();
    error PassiveReturnProhibitedRequiresActiveContribution();
    error MonetaryDividendsAndFiatRailsProhibited();
    error InvalidAssetOrCapacity();
    error InsufficientUtilityCredits();

    modifier onlyGovernance() {
        if (msg.sender != governanceAuthority) revert OnlyGovernance();
        _;
    }

    modifier onlyVerifier() {
        if (!authorizedVerifiers[msg.sender] && msg.sender != governanceAuthority) {
            revert OnlyAuthorizedVerifier();
        }
        _;
    }

    constructor(address _governanceAuthority) {
        governanceAuthority = _governanceAuthority == address(0) ? msg.sender : _governanceAuthority;
        authorizedVerifiers[governanceAuthority] = true;
    }

    /**
     * @notice Explicitly reject any native token or fiat-pegged value deposits to enforce
     *         non-dividend utility operation and BSP Circular No. 1108 VASP separation.
     */
    receive() external payable {
        revert MonetaryDividendsAndFiatRailsProhibited();
    }

    function setAuthorizedVerifier(address verifier, bool status) external onlyGovernance {
        authorizedVerifiers[verifier] = status;
    }

    function registerNationalAsset(
        bytes32 assetId,
        string calldata assetName,
        string calldata locationRegion,
        UtilityCreditType creditType,
        uint256 totalCapacityUnits
    ) external onlyGovernance {
        if (assetId == bytes32(0) || totalCapacityUnits == 0 || assets[assetId].active) {
            revert InvalidAssetOrCapacity();
        }
        assets[assetId] = NationalAsset({
            assetId: assetId,
            assetName: assetName,
            locationRegion: locationRegion,
            creditType: creditType,
            totalCapacityUnits: totalCapacityUnits,
            allocatedUnits: 0,
            active: true
        });
        assetIds.push(assetId);
        emit AssetRegistered(assetId, assetName, creditType, totalCapacityUnits);
    }

    /**
     * @notice Records verifiable active contribution (clean energy kWh, crop submission, or governance vote).
     *         Required to unlock staking boosters and functional RWA utility credits.
     */
    function recordActiveContribution(
        address participant,
        ActivityContributionType activityType,
        uint256 quantity,
        bytes32 proofHash
    ) external onlyVerifier {
        if (participant == address(0) || quantity == 0 || proofHash == bytes32(0)) {
            revert PassiveReturnProhibitedRequiresActiveContribution();
        }

        ActivityRecord storage rec = participantActivity[participant];
        uint256 pointsAdded = 0;

        if (activityType == ActivityContributionType.CLEAN_ENERGY_GENERATION) {
            rec.cleanEnergyKwhGenerated += quantity;
            pointsAdded = quantity * 2;
        } else if (activityType == ActivityContributionType.CROP_HARVEST_SUBMISSION) {
            rec.cropBatchesSubmitted += quantity;
            pointsAdded = quantity * 10;
        } else if (activityType == ActivityContributionType.GOVERNANCE_PROPOSAL_VOTE) {
            rec.governanceVotesCast += quantity;
            pointsAdded = quantity * 5;
        } else {
            pointsAdded = quantity * 5;
        }

        rec.totalActivityPoints += pointsAdded;
        rec.lastActivityProofHash = proofHash;
        rec.lastUpdatedBlock = uint64(block.number);

        emit ActivityContributionVerified(
            participant,
            activityType,
            quantity,
            rec.totalActivityPoints,
            proofHash
        );
    }

    function allocateUtilityUnits(
        bytes32 assetId,
        address participant,
        uint256 units
    ) external onlyGovernance {
        NationalAsset storage asset = assets[assetId];
        if (!asset.active || asset.allocatedUnits + units > asset.totalCapacityUnits) {
            revert InvalidAssetOrCapacity();
        }
        asset.allocatedUnits += units;
        utilityUnits[assetId][participant] += units;
        emit UtilityUnitsAllocated(assetId, participant, units);
    }

    /**
     * @notice Claims non-dividend functional utility discounts (e.g., warehouse storage credits
     *         or solar kWh discounts). Strictly gated by active contribution points — passive
     *         holders with zero activity points cannot claim utility credits.
     */
    function claimFunctionalUtilityCredits(
        bytes32 assetId,
        address participant
    ) external returns (uint256 issuedCredits) {
        NationalAsset storage asset = assets[assetId];
        if (!asset.active) revert InvalidAssetOrCapacity();

        ActivityRecord storage act = participantActivity[participant];
        uint256 unitsHeld = utilityUnits[assetId][participant];

        // SEC Mitigation: Passive holding yields 0. Active contribution points are mandatory.
        if (act.totalActivityPoints == 0 || unitsHeld == 0) {
            revert PassiveReturnProhibitedRequiresActiveContribution();
        }

        issuedCredits = (unitsHeld * act.totalActivityPoints) / 100;
        if (issuedCredits == 0) {
            issuedCredits = 1;
        }

        // Consume activity points upon credit issuance so rewards remain strictly tied to ongoing activity
        uint256 appliedPoints = act.totalActivityPoints;
        act.totalActivityPoints = 0;

        functionalUtilityCredits[assetId][participant] += issuedCredits;

        emit FunctionalUtilityCreditIssued(
            assetId,
            participant,
            asset.creditType,
            issuedCredits,
            appliedPoints
        );
    }

    /**
     * @notice Redeems functional utility discounts for physical infrastructure usage
     *         (e.g., warehouse space or solar energy offset).
     */
    function redeemFunctionalUtilityCredits(
        bytes32 assetId,
        uint256 creditsToRedeem,
        bytes32 serviceReceiptHash
    ) external {
        NationalAsset storage asset = assets[assetId];
        if (!asset.active || serviceReceiptHash == bytes32(0)) revert InvalidAssetOrCapacity();
        uint256 bal = functionalUtilityCredits[assetId][msg.sender];
        if (bal < creditsToRedeem || creditsToRedeem == 0) revert InsufficientUtilityCredits();

        functionalUtilityCredits[assetId][msg.sender] = bal - creditsToRedeem;

        emit FunctionalUtilityCreditRedeemed(
            assetId,
            msg.sender,
            asset.creditType,
            creditsToRedeem,
            serviceReceiptHash
        );
    }
}

