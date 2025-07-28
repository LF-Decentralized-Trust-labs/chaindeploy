-- Remove deployment mode and network configuration fields from prometheus_config
ALTER TABLE prometheus_config 
DROP CONSTRAINT IF EXISTS check_deployment_mode,
DROP CONSTRAINT IF EXISTS check_network_mode;

ALTER TABLE prometheus_config 
DROP COLUMN IF EXISTS deployment_mode,
DROP COLUMN IF EXISTS network_mode,
DROP COLUMN IF EXISTS extra_hosts,
DROP COLUMN IF EXISTS restart_policy,
DROP COLUMN IF EXISTS container_name,
DROP COLUMN IF EXISTS service_name,
DROP COLUMN IF EXISTS service_user,
DROP COLUMN IF EXISTS service_group,
DROP COLUMN IF EXISTS data_dir,
DROP COLUMN IF EXISTS config_dir,
DROP COLUMN IF EXISTS binary_path; 