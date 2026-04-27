-- Add an explicit pointer from node_groups to the POSTGRES service they
-- depend on. Previously the link went the other way (services.node_group_id),
-- but that model conflates "this service is attached to a group" with
-- "services are a property of a group". Services are a standalone resource
-- — a committer group references the one it needs, not vice versa.
--
-- The legacy services.node_group_id column is kept for one release so
-- existing data keeps working while callers migrate to the new pointer.
-- A future migration will drop it.
ALTER TABLE node_groups ADD COLUMN postgres_service_id INTEGER REFERENCES services(id);
CREATE INDEX idx_node_groups_postgres_service_id ON node_groups(postgres_service_id);
