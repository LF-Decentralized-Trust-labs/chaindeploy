-- Add session_id column to audit_logs table
ALTER TABLE audit_logs ADD COLUMN session_id TEXT;

-- Create index for session_id to improve query performance
CREATE INDEX IF NOT EXISTS idx_audit_logs_session_id ON audit_logs(session_id); 