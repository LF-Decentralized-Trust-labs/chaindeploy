-- Reverse of 0022_add_node_groups_and_services.up.sql.

DROP INDEX IF EXISTS idx_service_events_service_id;
DROP TABLE IF EXISTS service_events;

DROP INDEX IF EXISTS idx_service_backups_status;
DROP INDEX IF EXISTS idx_service_backups_service_id;
DROP TABLE IF EXISTS service_backups;

DROP INDEX IF EXISTS idx_services_status;
DROP INDEX IF EXISTS idx_services_service_type;
DROP INDEX IF EXISTS idx_services_node_group_id;
DROP TABLE IF EXISTS services;

DELETE FROM node_statuses WHERE name IN ('DEGRADED', 'CREATED');

DELETE FROM node_types WHERE name IN (
    'FABRICX_ORDERER_ROUTER',
    'FABRICX_ORDERER_BATCHER',
    'FABRICX_ORDERER_CONSENTER',
    'FABRICX_ORDERER_ASSEMBLER',
    'FABRICX_COMMITTER_SIDECAR',
    'FABRICX_COMMITTER_COORDINATOR',
    'FABRICX_COMMITTER_VALIDATOR',
    'FABRICX_COMMITTER_VERIFIER',
    'FABRICX_COMMITTER_QUERY_SERVICE'
);

DROP INDEX IF EXISTS idx_nodes_node_group_id;
ALTER TABLE nodes DROP COLUMN node_group_id;

DROP INDEX IF EXISTS idx_node_groups_status;
DROP INDEX IF EXISTS idx_node_groups_group_type;
DROP INDEX IF EXISTS idx_node_groups_platform;
DROP TABLE IF EXISTS node_groups;
