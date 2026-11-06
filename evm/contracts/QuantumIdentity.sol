// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QuantumIdentity
 * @notice Cryptographic Agility & Post-Quantum Public Key Registry for BETTERGOV Chain.
 * @dev Maps identity addresses and cryptographic algorithm identifiers to flexible public key
 *      storage slots (`bytes publicKey`). Supports classical (ECDSA secp256k1, Ed25519) and
 *      NIST-standardized post-quantum signature schemes (CRYSTALS-Dilithium / ML-DSA,
 *      Falcon / FN-DSA, and SPHINCS+ / SLH-DSA) with pluggable EVM precompile verifiers.
 */
contract QuantumIdentity {
    enum AlgorithmType {
        ECDSA_SECP256K1,     // Classical Ethereum 65-byte ECDSA
        ED25519,             // Classical CometBFT consensus 32-byte pubkey
        DILITHIUM_ML_DSA_65, // NIST FIPS 204 Lattice-based Post-Quantum Signature
        FALCON_512,          // NIST FN-DSA Lattice-based Compact Post-Quantum Signature
        FALCON_1024,         // NIST FN-DSA High-Security Post-Quantum Signature
        SPHINCS_PLUS_SHA256  // NIST FIPS 205 Stateless Hash-Based Post-Quantum Signature
    }

    struct AgileKeySlot {
        AlgorithmType algorithm;
        bytes publicKey;
        bytes32 keyCommitmentHash;
        uint64 keyEpoch;
        uint64 registeredAt;
        bool active;
        bool revoked;
    }

    address public governanceAuthority;

    // subjectAddress => AlgorithmType => AgileKeySlot
    mapping(address => mapping(AlgorithmType => AgileKeySlot)) private keySlots;
    // subjectAddress => preferred primary algorithm for authentication
    mapping(address => AlgorithmType) public preferredAlgorithm;
    // AlgorithmType => EVM Precompile or Verifier Contract Address
    mapping(AlgorithmType => address) public algorithmPrecompileVerifiers;

    event AgileKeyRegistered(
        address indexed subject,
        AlgorithmType indexed algorithm,
        bytes32 indexed keyCommitmentHash,
        uint64 keyEpoch,
        uint256 publicKeyLength
    );
    event AgileKeyRevoked(
        address indexed subject,
        AlgorithmType indexed algorithm,
        uint64 keyEpoch
    );
    event PreferredAlgorithmUpdated(address indexed subject, AlgorithmType indexed algorithm);
    event PostQuantumPrecompileConfigured(
        AlgorithmType indexed algorithm,
        address indexed precompileAddress
    );

    error OnlyGovernance();
    error InvalidPublicKeyLength();
    error KeySlotNotActive();
    error PrecompileVerifierNotConfigured();

    modifier onlyGovernance() {
        if (msg.sender != governanceAuthority) revert OnlyGovernance();
        _;
    }

    constructor(address _governanceAuthority) {
        governanceAuthority = _governanceAuthority == address(0) ? msg.sender : _governanceAuthority;
    }

    /**
     * @notice Allows protocol governance to bind an AlgorithmType (e.g., Dilithium, Falcon, SPHINCS+)
     *         to its EVM precompile or verifier address as soon as activated on the network.
     */
    function setAlgorithmPrecompileVerifier(
        AlgorithmType algorithm,
        address precompileAddress
    ) external onlyGovernance {
        algorithmPrecompileVerifiers[algorithm] = precompileAddress;
        emit PostQuantumPrecompileConfigured(algorithm, precompileAddress);
    }

    /**
     * @notice Registers or rotates a public key in the caller's agile key slot for a given algorithm.
     */
    function registerAgilePublicKey(
        AlgorithmType algorithm,
        bytes calldata publicKey,
        bool setAsPreferred
    ) external returns (bytes32 commitmentHash) {
        if (publicKey.length < 32 || publicKey.length > 4096) {
            revert InvalidPublicKeyLength();
        }

        commitmentHash = keccak256(publicKey);
        AgileKeySlot storage slot = keySlots[msg.sender][algorithm];
        uint64 nextEpoch = slot.keyEpoch + 1;

        slot.algorithm = algorithm;
        slot.publicKey = publicKey;
        slot.keyCommitmentHash = commitmentHash;
        slot.keyEpoch = nextEpoch;
        slot.registeredAt = uint64(block.timestamp);
        slot.active = true;
        slot.revoked = false;

        if (setAsPreferred) {
            preferredAlgorithm[msg.sender] = algorithm;
            emit PreferredAlgorithmUpdated(msg.sender, algorithm);
        }

        emit AgileKeyRegistered(
            msg.sender,
            algorithm,
            commitmentHash,
            nextEpoch,
            publicKey.length
        );
    }

    /**
     * @notice Revokes an active key slot (e.g. if a classical key is retired during post-quantum migration).
     */
    function revokeAgilePublicKey(AlgorithmType algorithm) external {
        AgileKeySlot storage slot = keySlots[msg.sender][algorithm];
        if (!slot.active) revert KeySlotNotActive();
        slot.active = false;
        slot.revoked = true;
        emit AgileKeyRevoked(msg.sender, algorithm, slot.keyEpoch);
    }

    /**
     * @notice Returns the key metadata and raw public key bytes for a subject's algorithm slot.
     */
    function getKeySlot(
        address subject,
        AlgorithmType algorithm
    )
        external
        view
        returns (
            bytes memory publicKey,
            bytes32 keyCommitmentHash,
            uint64 keyEpoch,
            uint64 registeredAt,
            bool active,
            bool revoked
        )
    {
        AgileKeySlot storage s = keySlots[subject][algorithm];
        return (
            s.publicKey,
            s.keyCommitmentHash,
            s.keyEpoch,
            s.registeredAt,
            s.active,
            s.revoked
        );
    }

    /**
     * @notice Verifies a signature using the registered precompile for the specified algorithm
     *         (Dilithium, Falcon, SPHINCS+, or Ed25519).
     */
    function verifyWithAgilePrecompile(
        address subject,
        AlgorithmType algorithm,
        bytes32 messageDigest,
        bytes calldata signature
    ) external view returns (bool valid) {
        AgileKeySlot storage s = keySlots[subject][algorithm];
        if (!s.active || s.revoked) revert KeySlotNotActive();

        address verifier = algorithmPrecompileVerifiers[algorithm];
        if (verifier == address(0)) revert PrecompileVerifierNotConfigured();

        (bool ok, bytes memory out) = verifier.staticcall(
            abi.encode(s.publicKey, messageDigest, signature)
        );
        if (!ok || out.length < 32) {
            return false;
        }
        return abi.decode(out, (bool));
    }
}

