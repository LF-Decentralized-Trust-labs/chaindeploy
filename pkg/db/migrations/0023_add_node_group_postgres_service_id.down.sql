DROP INDEX IF EXISTS idx_node_groups_postgres_service_id;
ALTER TABLE node_groups DROP COLUMN postgres_service_id;
