-- Add missing indexes for common query patterns

-- nodes table: frequently filtered by platform, network_id, status
CREATE INDEX IF NOT EXISTS idx_nodes_platform ON nodes(platform);
CREATE INDEX IF NOT EXISTS idx_nodes_network_id ON nodes(network_id);
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_node_type ON nodes(node_type);

-- keys table: filtered by provider_id in GetAllKeys, GetKeyCountByProvider
CREATE INDEX IF NOT EXISTS idx_keys_provider_id ON keys(provider_id);
CREATE INDEX IF NOT EXISTS idx_keys_status ON keys(status);
CREATE INDEX IF NOT EXISTS idx_keys_ethereum_address ON keys(ethereum_address);

-- fabric_organizations table: looked up by msp_id frequently
CREATE INDEX IF NOT EXISTS idx_fabric_organizations_msp_id ON fabric_organizations(msp_id);

-- node_keys table: queried by node_id and key_id
CREATE INDEX IF NOT EXISTS idx_node_keys_node_id ON node_keys(node_id);
CREATE INDEX IF NOT EXISTS idx_node_keys_key_id ON node_keys(key_id);

-- backups table: queried by created_at range
CREATE INDEX IF NOT EXISTS idx_backups_created_at ON backups(created_at);

-- fabric_chaincodes table: queried by network_id
CREATE INDEX IF NOT EXISTS idx_fabric_chaincodes_network_id ON fabric_chaincodes(network_id);
