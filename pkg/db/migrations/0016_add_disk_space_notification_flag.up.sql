-- Add disk space warning notification flag to notification_providers table
ALTER TABLE notification_providers ADD COLUMN notify_disk_space_warning BOOLEAN NOT NULL DEFAULT false;
