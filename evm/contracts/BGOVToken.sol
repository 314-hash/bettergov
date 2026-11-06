// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title BGOVToken
 * @notice Official ERC-20 Smart Contract Wrapper & Utility Interface for Native BetterGov (BGOV).
 * @dev On BETTERGOV Chain (EVM Chain ID 31337 / 77002), BGOV is the native Layer 1 gas and PoS
 *      staking coin (implemented at the protocol level in `chain/x/bank/bank.go`).
 *      This smart contract provides the canonical ERC-20 interface (18 decimals, symbol "BGOV")
 *      so wallets (MetaMask "Import Tokens"), smart contracts, and public treasury modules
 *      can interact with BGOV via standard ERC-20 calls (`balanceOf`, `transfer`, `approve`,
 *      `transferFrom`, `deposit`, `withdraw`).
 *
 *      Regulatory Posture:
 *      - BSP Circular No. 1108: Strictly operates at the crypto-utility layer (zero PHP fiat rails).
 *      - SEC Classification Mitigation: Pure utility & governance token; zero passive dividend loops.
 */
contract BGOVToken {
    string public constant name = "BetterGov";
    string public constant symbol = "BGOV";
    uint8 public constant decimals = 18;

    string public constant BSP_1108_COMPLIANCE =
        "CRYPTO_UTILITY_LAYER_ONLY: ZERO_DIRECT_PHP_FIAT_RAILS";

    uint256 public totalSupply;
    address public governanceAuthority;
    uint256 private unlocked = 1;

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event Deposit(address indexed dst, uint256 amount);
    event Withdrawal(address indexed src, uint256 amount);

    error OnlyGovernance();
    error InsufficientBalance();
    error InsufficientAllowance();
    error ZeroAddressProhibited();
    error ReentrancyBlocked();
    error NativeTransferFailed();

    modifier nonReentrant() {
        if (unlocked != 1) revert ReentrancyBlocked();
        unlocked = 2;
        _;
        unlocked = 1;
    }

    modifier onlyGovernance() {
        if (msg.sender != governanceAuthority) revert OnlyGovernance();
        _;
    }

    constructor(address _governanceAuthority) {
        governanceAuthority = _governanceAuthority == address(0) ? msg.sender : _governanceAuthority;

        // Mirror the exact Layer 1 Genesis Allocations (85,000,000 BGOV total)
        _mintGenesis(0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266, 25_000_000 * (10 ** uint256(decimals))); // Deployer / Auditor
        _mintGenesis(0xa86c5d224FA577D40E52647Dbc644F552cebeF58, 20_000_000 * (10 ** uint256(decimals))); // On-Chain TreasuryVault Pool
        _mintGenesis(0x70997970C51812dc3A010C7d01b50e0d17dc79C8, 10_000_000 * (10 ** uint256(decimals))); // Validator 1 (Sentinel-Alpha-Node)
        _mintGenesis(0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC, 10_000_000 * (10 ** uint256(decimals))); // Validator 2 (CivicTrust-Validator)
        _mintGenesis(0x90F79bf6EB2c4f870365E785982E1f101E93b906, 10_000_000 * (10 ** uint256(decimals))); // Validator 3 (Aegis-GovTech-Staking)
        _mintGenesis(0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65, 10_000_000 * (10 ** uint256(decimals))); // Validator 4 (OpenLedger-Guard)
    }

    function _mintGenesis(address account, uint256 amount) internal {
        totalSupply += amount;
        balanceOf[account] += amount;
        emit Transfer(address(0), account, amount);
    }

    /**
     * @notice Wraps native L1 BGOV 1:1 into ERC-20 BGOV when sent to the contract.
     */
    receive() external payable {
        deposit();
    }

    /**
     * @notice Deposits native L1 BGOV and mints 1:1 ERC-20 BGOV balance to caller.
     */
    function deposit() public payable {
        balanceOf[msg.sender] += msg.value;
        totalSupply += msg.value;
        emit Deposit(msg.sender, msg.value);
        emit Transfer(address(0), msg.sender, msg.value);
    }

    /**
     * @notice Burns ERC-20 BGOV balance and unwraps 1:1 back to native L1 BGOV.
     */
    function withdraw(uint256 amount) external nonReentrant {
        if (balanceOf[msg.sender] < amount) revert InsufficientBalance();
        balanceOf[msg.sender] -= amount;
        totalSupply -= amount;

        (bool ok, ) = msg.sender.call{value: amount}("");
        if (!ok) revert NativeTransferFailed();

        emit Withdrawal(msg.sender, amount);
        emit Transfer(msg.sender, address(0), amount);
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        if (spender == address(0)) revert ZeroAddressProhibited();
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        return _transfer(msg.sender, to, amount);
    }

    function transferFrom(
        address from,
        address to,
        uint256 amount
    ) external returns (bool) {
        uint256 currentAllowance = allowance[from][msg.sender];
        if (currentAllowance != type(uint256).max) {
            if (currentAllowance < amount) revert InsufficientAllowance();
            allowance[from][msg.sender] = currentAllowance - amount;
        }
        return _transfer(from, to, amount);
    }

    function _transfer(
        address from,
        address to,
        uint256 amount
    ) internal returns (bool) {
        if (to == address(0)) revert ZeroAddressProhibited();
        if (balanceOf[from] < amount) revert InsufficientBalance();

        balanceOf[from] -= amount;
        balanceOf[to] += amount;

        emit Transfer(from, to, amount);
        return true;
    }
}
