-- Node groups: logical parents that own shared identity/crypto for a set of
-- child nodes (e.g. a FabricX orderer group owns router/batcher/consenter/
-- assembler children). A node_group is not a runnable container — it is
-- metadata plus a lifecycle coordinator.
CREATE TABLE node_groups (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL UNIQUE,
    platform           TEXT NOT NULL REFERENCES blockchain_platforms(name),
    group_type         TEXT NOT NULL,
    msp_id             TEXT,
    organization_id    INTEGER REFERENCES fabric_organizations(id),
    party_id           INTEGER,
    version            TEXT,
    external_ip        TEXT,
    domain_names       TEXT,
    sign_key_id        INTEGER,
    tls_key_id         INTEGER,
    sign_cert          TEXT,
    tls_cert           TEXT,
    ca_cert            TEXT,
    tls_ca_cert        TEXT,
    config             TEXT,
    deployment_config  TEXT,
    status             TEXT NOT NULL DEFAULT 'CREATED' REFERENCES node_statuses(name),
    error_message      TEXT,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP
);

CREATE INDEX idx_node_groups_platform ON node_groups(platform);
CREATE INDEX idx_node_groups_group_type ON node_groups(group_type);
CREATE INDEX idx_node_groups_status ON node_groups(status);

-- Child nodes reference their group.
ALTER TABLE nodes ADD COLUMN node_group_id INTEGER REFERENCES node_groups(id) ON DELETE CASCADE;
CREATE INDEX idx_nodes_node_group_id ON nodes(node_group_id);

-- New per-role child node types for FabricX.
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_ORDERER_ROUTER');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_ORDERER_BATCHER');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_ORDERER_CONSENTER');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_ORDERER_ASSEMBLER');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_COMMITTER_SIDECAR');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_COMMITTER_COORDINATOR');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_COMMITTER_VALIDATOR');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_COMMITTER_VERIFIER');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_COMMITTER_QUERY_SERVICE');

-- New group-level status values used for aggregated group state.
-- CREATED is the default status for newly-persisted node_groups/services
-- rows before any lifecycle action runs (see DEFAULT 'CREATED' above).
INSERT OR IGNORE INTO node_statuses (name) VALUES ('CREATED');
INSERT OR IGNORE INTO node_statuses (name) VALUES ('DEGRADED');

-- Services: managed supporting infrastructure attached to a node_group
-- (typically) or standalone. First citizen is POSTGRES for FabricX committers.
CREATE TABLE services (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    node_group_id      INTEGER REFERENCES node_groups(id) ON DELETE CASCADE,
    name               TEXT NOT NULL UNIQUE,
    service_type       TEXT NOT NULL,
    version            TEXT,
    status             TEXT NOT NULL DEFAULT 'CREATED' REFERENCES node_statuses(name),
    config             TEXT,
    deployment_config  TEXT,
    backup_target_id   INTEGER REFERENCES backup_targets(id),
    backup_config      TEXT,
    error_message      TEXT,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP
);

CREATE INDEX idx_services_node_group_id ON services(node_group_id);
CREATE INDEX idx_services_service_type ON services(service_type);
CREATE INDEX idx_services_status ON services(status);

-- Individual backup records for a service. For postgres+WAL-G these are base
-- backups only; WAL segments stream continuously and are tracked by WAL-G
-- itself, not row-by-row here.
CREATE TABLE service_backups (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id         INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    backup_type        TEXT NOT NULL,
    s3_key             TEXT,
    size_bytes         INTEGER,
    lsn                TEXT,
    timeline           INTEGER,
    status             TEXT NOT NULL,
    started_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at       TIMESTAMP,
    error_message      TEXT,
    metadata           TEXT
);

CREATE INDEX idx_service_backups_service_id ON service_backups(service_id);
CREATE INDEX idx_service_backups_status ON service_backups(status);

-- Lifecycle event log per service, mirroring node_events semantics.
CREATE TABLE service_events (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id         INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    type               TEXT NOT NULL,
    status             TEXT NOT NULL,
    data               TEXT,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_service_events_service_id ON service_events(service_id);
