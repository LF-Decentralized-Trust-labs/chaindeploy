-- Remove disk space warning notification flag from notification_providers table
ALTER TABLE notification_providers DROP COLUMN notify_disk_space_warning;
