-- Remove session_id index
DROP INDEX IF EXISTS idx_audit_logs_session_id;

-- Remove session_id column
ALTER TABLE audit_logs DROP COLUMN session_id; 