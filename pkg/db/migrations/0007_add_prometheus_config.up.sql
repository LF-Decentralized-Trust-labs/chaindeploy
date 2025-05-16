CREATE TABLE prometheus_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    prometheus_port INTEGER NOT NULL,
    data_dir TEXT NOT NULL,
    config_dir TEXT NOT NULL,
    container_name TEXT NOT NULL,
    scrape_interval INTEGER NOT NULL,
    evaluation_interval INTEGER NOT NULL,
    deployment_mode TEXT NOT NULL DEFAULT 'docker',
    docker_image TEXT NOT NULL DEFAULT 'prom/prometheus:latest',
    docker_network TEXT,
    docker_restart_policy TEXT NOT NULL DEFAULT 'unless-stopped',
    docker_extra_args TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default configuration
INSERT INTO prometheus_config (
    prometheus_port,
    data_dir,
    config_dir,
    container_name,
    scrape_interval,
    evaluation_interval,
    deployment_mode,
    docker_image,
    docker_network,
    docker_restart_policy,
    docker_extra_args
) VALUES (
    9090,
    '/var/lib/prometheus',
    '/etc/prometheus',
    'chainlaunch-prometheus',
    15,
    15,
    'docker',
    'prom/prometheus:latest',
    'chainlaunch-network',
    'unless-stopped',
    '--web.enable-lifecycle --web.enable-admin-api'
);
