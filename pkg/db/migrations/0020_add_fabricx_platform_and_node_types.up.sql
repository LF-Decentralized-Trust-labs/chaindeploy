-- FabricX platform and node types
-- Required by the FabricX node creation flow (pkg/nodes/types/types.go).
-- Without these rows, FOREIGN KEY constraint failures occur on the
-- nodes.platform → blockchain_platforms.name and
-- nodes.node_type → node_types.name references.
INSERT OR IGNORE INTO blockchain_platforms (name) VALUES ('FABRICX');
-- Lowercase variant used by the network service (NetworkTypeFabricX = "fabricx").
INSERT OR IGNORE INTO blockchain_platforms (name) VALUES ('fabricx');

-- Endorsement in FabricX is handled by token-sdk-x, not chaindeploy, so
-- there is no FABRICX_ENDORSER node type.
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_ORDERER_GROUP');
INSERT OR IGNORE INTO node_types (name) VALUES ('FABRICX_COMMITTER');
