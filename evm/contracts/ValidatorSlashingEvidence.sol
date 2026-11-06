// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title ValidatorSlashingEvidence
 * @notice On-chain evidence submission bridge for reporting double-signing equivocation
 *         to the BETTERGOV native x/slashing consensus module.
 * @dev Does NOT replace native protocol-level PoS staking or slashing; acts as an auditable
 *      EVM receipt ledger for evidence submitted by watchtowers and Validator Guardian agents.
 */
contract ValidatorSlashingEvidence {
    struct EquivocationReceipt {
        bytes32 evidenceHash;
        bytes32 consensusPubkeyHash;
        uint64 height;
        uint32 round;
        bytes32 blockHashA;
        bytes32 blockHashB;
        address reporter;
        uint64 reportedAt;
    }

    mapping(bytes32 => EquivocationReceipt) public evidenceReceipts;
    bytes32[] public evidenceList;

    event DoubleSignEvidenceSubmitted(
        bytes32 indexed evidenceHash,
        bytes32 indexed consensusPubkeyHash,
        uint64 height,
        uint32 round,
        bytes32 blockHashA,
        bytes32 blockHashB,
        address indexed reporter
    );

    error InvalidEquivocationParameters();
    error DuplicateEvidence();

    function submitDoubleSignEvidence(
        bytes32 consensusPubkeyHash,
        uint64 height,
        uint32 round,
        bytes32 blockHashA,
        bytes32 blockHashB
    ) external returns (bytes32 evidenceHash) {
        if (
            consensusPubkeyHash == bytes32(0) ||
            height == 0 ||
            blockHashA == bytes32(0) ||
            blockHashB == bytes32(0) ||
            blockHashA == blockHashB
        ) {
            revert InvalidEquivocationParameters();
        }

        evidenceHash = keccak256(
            abi.encodePacked(consensusPubkeyHash, height, round, blockHashA, blockHashB)
        );
        if (evidenceReceipts[evidenceHash].reportedAt != 0) {
            revert DuplicateEvidence();
        }

        evidenceReceipts[evidenceHash] = EquivocationReceipt({
            evidenceHash: evidenceHash,
            consensusPubkeyHash: consensusPubkeyHash,
            height: height,
            round: round,
            blockHashA: blockHashA,
            blockHashB: blockHashB,
            reporter: msg.sender,
            reportedAt: uint64(block.timestamp)
        });
        evidenceList.push(evidenceHash);

        emit DoubleSignEvidenceSubmitted(
            evidenceHash,
            consensusPubkeyHash,
            height,
            round,
            blockHashA,
            blockHashB,
            msg.sender
        );
    }

    function totalEvidenceCount() external view returns (uint256) {
        return evidenceList.length;
    }
}

