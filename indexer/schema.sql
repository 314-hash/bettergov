-- BETTERGOV Blockchain PostgreSQL Indexer Schema

CREATE TABLE IF NOT EXISTS blocks (
    height BIGINT PRIMARY KEY,
    chain_id VARCHAR(64) NOT NULL,
    block_hash VARCHAR(66) NOT NULL UNIQUE,
    app_hash VARCHAR(66) NOT NULL,
    proposer_address VARCHAR(80) NOT NULL,
    tx_count INT NOT NULL DEFAULT 0,
    gas_used BIGINT NOT NULL DEFAULT 0,
    block_time TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS transactions (
    tx_hash VARCHAR(66) PRIMARY KEY,
    block_height BIGINT NOT NULL REFERENCES blocks(height),
    sender VARCHAR(64) NOT NULL,
    recipient VARCHAR(64),
    contract_address VARCHAR(64),
    value_abgov NUMERIC(78, 0) NOT NULL DEFAULT 0,
    gas_used BIGINT NOT NULL,
    effective_gas_price VARCHAR(32) NOT NULL,
    status SMALLINT NOT NULL,
    input_data TEXT
);

CREATE TABLE IF NOT EXISTS validators (
    operator_address VARCHAR(64) PRIMARY KEY,
    consensus_address VARCHAR(80) NOT NULL UNIQUE,
    moniker VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    jailed BOOLEAN NOT NULL DEFAULT FALSE,
    tombstoned BOOLEAN NOT NULL DEFAULT FALSE,
    tokens_abgov NUMERIC(78, 0) NOT NULL,
    commission_rate_bps INT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS governance_proposals (
    proposal_id BIGINT PRIMARY KEY,
    title VARCHAR(256) NOT NULL,
    proposal_type VARCHAR(64) NOT NULL,
    proposer VARCHAR(64) NOT NULL,
    status VARCHAR(64) NOT NULL,
    total_deposit_abgov NUMERIC(78, 0) NOT NULL,
    voting_end_time TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS transparency_records (
    record_id VARCHAR(128) PRIMARY KEY,
    category VARCHAR(64) NOT NULL,
    title VARCHAR(256) NOT NULL,
    agency_or_dept VARCHAR(128) NOT NULL,
    sha256_hash VARCHAR(64) NOT NULL,
    verification_status VARCHAR(64) NOT NULL,
    disclaimer TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tx_sender ON transactions(sender);
CREATE INDEX IF NOT EXISTS idx_tx_recipient ON transactions(recipient);
CREATE INDEX IF NOT EXISTS idx_transparency_hash ON transparency_records(sha256_hash);

