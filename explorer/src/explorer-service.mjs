/**
 * BETTERGOV Block Explorer Service
 * Provides unified queries for latest blocks, transactions, wallets, BGOV balances,
 * validator profiles, delegations, smart contracts, proposals, and gas metrics.
 */

export class ExplorerService {
  constructor(indexer) {
    this.indexer = indexer;
    this.indexer.seedGenesisState();
  }

  getOverviewMetrics() {
    const totalBonded = this.indexer.validators.reduce((s, v) => s + v.tokensBgov, 0);
    return {
      chainId: 'bettergov-devnet-1',
      evmChainId: 31337,
      nativeSymbol: 'BGOV',
      decimals: 18,
      latestBlockHeight: this.indexer.lastSyncedHeight,
      totalSupplyBgov: 85000000,
      totalBondedBgov: totalBonded,
      treasuryBalanceBgov: 20000000,
      activeValidatorsCount: this.indexer.validators.filter((v) => !v.jailed).length,
      averageBlockTimeSeconds: 3.0,
      baseGasPriceGwei: 1.0,
      totalTransparencyRecords: this.indexer.transparencyRecords.length,
    };
  }

  getBlocks() {
    return this.indexer.blocks;
  }

  getValidators() {
    return this.indexer.validators;
  }

  getSmartContracts() {
    return this.indexer.contracts;
  }

  getProposals() {
    return this.indexer.proposals;
  }

  getTransparencyRecords() {
    return this.indexer.transparencyRecords;
  }
}

