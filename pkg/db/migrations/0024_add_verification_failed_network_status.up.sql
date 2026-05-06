-- Add the 'verification_failed' status used by the FabricX network
-- post-provisioning verification step. The networks table reuses the
-- node_statuses lookup table for its status FK, so the value has to be
-- registered there before any UPDATE can reference it.
INSERT OR IGNORE INTO node_statuses (name) VALUES ('verification_failed');
