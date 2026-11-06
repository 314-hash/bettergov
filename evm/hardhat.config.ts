/**
 * Hardhat Configuration for BETTERGOV Layer 1 EVM Environment
 */
const config = {
  solidity: {
    version: "0.8.24",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200,
      },
    },
  },
  networks: {
    bettergovDevnet: {
      url: process.env.BETTERGOV_DEVNET_RPC_URL || "http://127.0.0.1:8545",
      chainId: 31337,
    },
    bettergovTestnet: {
      url: process.env.BETTERGOV_TESTNET_RPC_URL || "http://127.0.0.1:18545",
      chainId: 77002,
    },
  },
  paths: {
    sources: "./contracts",
    tests: "./test",
    cache: "./cache",
    artifacts: "./artifacts",
  },
};

export default config;

