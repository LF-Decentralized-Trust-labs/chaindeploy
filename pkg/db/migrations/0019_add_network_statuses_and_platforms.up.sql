-- Add lowercase platform values used by network creation code
INSERT OR IGNORE INTO blockchain_platforms (name) VALUES ('fabric');
INSERT OR IGNORE INTO blockchain_platforms (name) VALUES ('besu');

-- Add network-specific status values used by network deployers
-- These are stored in the node_statuses table since networks.status references it
INSERT OR IGNORE INTO node_statuses (name) VALUES ('creating');
INSERT OR IGNORE INTO node_statuses (name) VALUES ('genesis_block_created');
INSERT OR IGNORE INTO node_statuses (name) VALUES ('running');
INSERT OR IGNORE INTO node_statuses (name) VALUES ('stopped');
INSERT OR IGNORE INTO node_statuses (name) VALUES ('error');
INSERT OR IGNORE INTO node_statuses (name) VALUES ('proposed');
INSERT OR IGNORE INTO node_statuses (name) VALUES ('imported');
