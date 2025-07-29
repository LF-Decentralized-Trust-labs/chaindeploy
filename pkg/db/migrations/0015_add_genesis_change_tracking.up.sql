-- Add fields to track genesis block changes
ALTER TABLE networks ADD COLUMN genesis_changed_at TIMESTAMP;
ALTER TABLE networks ADD COLUMN genesis_changed_by INTEGER REFERENCES users(id);
ALTER TABLE networks ADD COLUMN genesis_change_reason TEXT; 