CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    event_source TEXT NOT NULL,
    user_identity INTEGER NOT NULL,
    source_ip TEXT,
    event_type TEXT NOT NULL,
    event_outcome TEXT NOT NULL,
    affected_resource TEXT,
    request_id TEXT,
    severity TEXT,
    details TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_identity ON audit_logs(user_identity);
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id);

-- Create trigger for updated_at
CREATE TRIGGER update_audit_logs_updated_at
AFTER UPDATE ON audit_logs
BEGIN
    UPDATE audit_logs SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END; 