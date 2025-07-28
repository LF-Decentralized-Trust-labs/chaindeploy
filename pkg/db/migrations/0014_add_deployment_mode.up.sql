
ALTER TABLE prometheus_config ADD COLUMN network_mode TEXT DEFAULT 'bridge';
ALTER TABLE prometheus_config ADD COLUMN extra_hosts TEXT;
ALTER TABLE prometheus_config ADD COLUMN restart_policy TEXT DEFAULT 'unless-stopped';
ALTER TABLE prometheus_config ADD COLUMN service_name TEXT DEFAULT 'chainlaunch-prometheus';
ALTER TABLE prometheus_config ADD COLUMN service_user TEXT DEFAULT 'prometheus';
ALTER TABLE prometheus_config ADD COLUMN service_group TEXT DEFAULT 'prometheus';
ALTER TABLE prometheus_config ADD COLUMN binary_path TEXT DEFAULT '/usr/local/bin/prometheus';
ALTER TABLE prometheus_config ADD COLUMN prometheus_version TEXT DEFAULT 'v3.3.1';
