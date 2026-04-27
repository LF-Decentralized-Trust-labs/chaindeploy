-- Fabric X namespaces: logical partitions within a FabricX channel.
-- Created by broadcasting an applicationpb.Tx that writes a NamespacePolicy
-- to the reserved _meta namespace on the channel.
CREATE TABLE fabricx_namespaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    network_id INTEGER NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT -1,
    submitter_msp_id TEXT NOT NULL,
    submitter_org_id INTEGER REFERENCES fabric_organizations(id),
    tx_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE(network_id, name)
);

CREATE INDEX idx_fabricx_namespaces_network ON fabricx_namespaces(network_id);
