-- name: GetNetworkByNetworkId :one
SELECT * FROM networks
WHERE network_id = ? LIMIT 1;

-- name: GetNetwork :one
SELECT * FROM networks
WHERE id = ? LIMIT 1;

-- name: ListNetworks :many
SELECT * FROM networks
ORDER BY created_at DESC;

-- name: CreateNetwork :one
INSERT INTO networks (
    name, platform, status, description, config,
    deployment_config, exposed_ports, domain, created_by, network_id
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: CreateNetworkFull :one
INSERT INTO networks (
    name, platform, status, description, config,
    deployment_config, exposed_ports, domain, created_by, network_id, genesis_block_b64
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;


-- name: GetNetworkByName :one
SELECT * FROM networks
WHERE name = ? LIMIT 1;

-- name: GetNode :one
SELECT * FROM nodes
WHERE id = ? LIMIT 1;

-- name: ListNodes :many
SELECT * FROM nodes
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountNodes :one
SELECT COUNT(*) FROM nodes;

-- name: ListNodesByNetwork :many
SELECT * FROM nodes
WHERE network_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;


-- name: ListNodesByPlatform :many
SELECT * FROM nodes
WHERE platform = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountNodesByPlatform :one
SELECT COUNT(*) FROM nodes
WHERE platform = ?;

-- name: CreateNode :one
INSERT INTO nodes (
    name,
    slug,
    platform,
    status,
    description,
    network_id,
    config,
    resources,
    endpoint,
    public_endpoint,
    p2p_address,
    created_by,
    fabric_organization_id,
    node_type,
    node_config,
    created_at,
    updated_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetFabricOrganization :one
SELECT * FROM fabric_organizations
WHERE id = ? LIMIT 1;

-- name: GetFabricOrganizationByMSPID :one
SELECT * FROM fabric_organizations
WHERE msp_id = ? LIMIT 1;

-- name: ListFabricOrganizations :many
SELECT * FROM fabric_organizations
ORDER BY created_at DESC;

-- name: CreateFabricOrganization :one
INSERT INTO fabric_organizations (
    msp_id, description, config, ca_config, sign_key_id,
    tls_root_key_id, provider_id, created_by,
    admin_tls_key_id, admin_sign_key_id, client_sign_key_id
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetKeyProviderByDefault :one
SELECT * FROM key_providers WHERE is_default = 1 LIMIT 1;

-- name: CreateKeyProvider :one
INSERT INTO key_providers (name, type, is_default, config)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetAllKeys :many
SELECT k.*, kp.name as provider_name, kp.type as provider_type
FROM keys k
JOIN key_providers kp ON k.provider_id = kp.id
WHERE (? IS NULL OR k.provider_id = ?);

-- name: CreateKey :one
INSERT INTO keys (
    name, description, algorithm, key_size, curve, format,
    public_key, private_key, certificate, status, expires_at, sha256_fingerprint,
    sha1_fingerprint, provider_id, user_id, is_ca, ethereum_address
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetKeyByID :one
SELECT k.*, kp.name as provider_name, kp.type as provider_type
FROM keys k
JOIN key_providers kp ON k.provider_id = kp.id
WHERE k.id = ?;

-- name: DeleteKey :exec
DELETE FROM keys WHERE id = ?;

-- name: ListKeyProviders :many
SELECT * FROM key_providers;

-- name: GetKeyProviderByID :one
SELECT * FROM key_providers WHERE id = ?;

-- name: DeleteKeyProvider :exec
DELETE FROM key_providers WHERE id = ?;

-- name: GetKeysCount :one
SELECT COUNT(*) FROM keys;

-- name: ListKeys :many
SELECT k.*, kp.name as provider_name, kp.type as provider_type
FROM keys k
JOIN key_providers kp ON k.provider_id = kp.id
ORDER BY k.created_at DESC
LIMIT ? OFFSET ?;

-- name: GetKey :one
SELECT k.*, kp.name as provider_name, kp.type as provider_type
FROM keys k
JOIN key_providers kp ON k.provider_id = kp.id
WHERE k.id = ?;

-- name: GetKeyProvider :one
SELECT * FROM key_providers WHERE id = ?;

-- name: UnsetDefaultProvider :exec
UPDATE key_providers SET is_default = 0 WHERE is_default = 1;

-- name: GetKeyCountByProvider :one
SELECT COUNT(*) FROM keys WHERE provider_id = ?;

-- name: UpdateKey :one
UPDATE keys
SET name = ?,
    description = ?,
    algorithm = ?,
    key_size = ?,
    curve = ?,
    format = ?,
    public_key = ?,
    private_key = ?,
    certificate = ?,
    status = ?,
    expires_at = ?,
    sha256_fingerprint = ?,
    sha1_fingerprint = ?,
    provider_id = ?,
    user_id = ?,
    ethereum_address = ?,
    updated_at = CURRENT_TIMESTAMP,
    signing_key_id = ?
WHERE id = ?
RETURNING *;

-- name: UpdateKeyProvider :one
UPDATE key_providers
SET name = ?,
    type = ?,
    is_default = ?,
    config = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateFabricOrganization :one
UPDATE fabric_organizations
SET description = ?
WHERE id = ?
RETURNING *;

-- name: DeleteFabricOrganization :exec
DELETE FROM fabric_organizations WHERE id = ?;

-- name: GetFabricOrganizationWithKeys :one
SELECT 
    fo.*,
    sk.public_key as sign_public_key,
    sk.certificate as sign_certificate,
    tk.public_key as tls_public_key,
    tk.certificate as tls_certificate,
    p.name as provider_name
FROM fabric_organizations fo
LEFT JOIN keys sk ON fo.sign_key_id = sk.id
LEFT JOIN keys tk ON fo.tls_root_key_id = tk.id
LEFT JOIN key_providers p ON fo.provider_id = p.id
WHERE fo.id = ?;

-- name: ListFabricOrganizationsWithKeys :many
SELECT 
    fo.*,
    sk.public_key as sign_public_key,
    sk.certificate as sign_certificate,
    tk.public_key as tls_public_key,
    tk.certificate as tls_certificate,
    p.name as provider_name
FROM fabric_organizations fo
LEFT JOIN keys sk ON fo.sign_key_id = sk.id
LEFT JOIN keys tk ON fo.tls_root_key_id = tk.id
LEFT JOIN key_providers p ON fo.provider_id = p.id
ORDER BY fo.created_at DESC;


-- name: UpdateNetworkGenesisBlock :one
UPDATE networks
SET genesis_block_b64 = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;


-- name: UpdateNodeConfig :one
UPDATE nodes
SET node_config = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateDeploymentConfig :one
UPDATE nodes
SET deployment_config = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;


-- name: UpdateNodeStatusWithError :one
UPDATE nodes
SET status = ?,
    error_message = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateNodeStatus :one
UPDATE nodes
SET status = ?,
    error_message = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteNode :exec
DELETE FROM nodes WHERE id = ?;

-- name: GetAllNodes :many
SELECT * FROM nodes;

-- name: GetNodeBySlug :one
SELECT * FROM nodes WHERE slug = ?;

-- name: CreateUser :one
INSERT INTO users (
    username, password, role, created_at, last_login_at, updated_at
) VALUES (
    ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id = ? LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = ? LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC;

-- name: UpdateUser :one
UPDATE users
SET username = ?,
    role = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateUserPassword :one
UPDATE users
SET password = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- name: UpdateUserLastLogin :one
UPDATE users
SET last_login_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateSession :one
INSERT INTO sessions (
  token,
  user_id,
  expires_at,
  session_id
) VALUES (
  ?, ?, ?, ?
)
RETURNING *;

-- name: GetSession :one
SELECT * FROM sessions WHERE token = ? LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = ?;


-- name: CreateNodeEvent :one
INSERT INTO node_events (
    node_id,
    event_type,
    description,
    data,
    status
) VALUES (
    ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetNodeEvent :one
SELECT * FROM node_events
WHERE id = ? LIMIT 1;

-- name: ListNodeEvents :many
SELECT * FROM node_events
WHERE node_id = ?
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountNodeEvents :one
SELECT COUNT(*) FROM node_events
WHERE node_id = ?;

-- name: GetLatestNodeEvent :one
SELECT * FROM node_events
WHERE node_id = ?
ORDER BY created_at DESC
LIMIT 1;

-- name: ListNodeEventsByType :many
SELECT * FROM node_events
WHERE node_id = ? AND event_type = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;



-- name: CountNetworks :one
SELECT COUNT(*) FROM networks;


-- name: DeleteNetwork :exec
DELETE FROM networks
WHERE id = ?;

-- name: UpdateNodeDeploymentConfig :one
UPDATE nodes
SET deployment_config = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: GetFabricOrganizationByID :one
SELECT * FROM fabric_organizations WHERE id = ? LIMIT 1;

-- name: GetFabricOrganizationByMspID :one
SELECT 
    fo.*,
    sk.public_key as sign_public_key,
    sk.certificate as sign_certificate,
    tk.public_key as tls_public_key,
    tk.certificate as tls_certificate,
    p.name as provider_name
FROM fabric_organizations fo
LEFT JOIN keys sk ON fo.sign_key_id = sk.id
LEFT JOIN keys tk ON fo.tls_root_key_id = tk.id
LEFT JOIN key_providers p ON fo.provider_id = p.id
WHERE fo.msp_id = ?;

-- name: UpdateNetworkStatus :exec
UPDATE networks
SET status = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;


-- Add queries for CRUD operations
-- name: CreateNetworkNode :one
INSERT INTO network_nodes (
    network_id,
    node_id,
    status,
    role
) VALUES (
    ?, ?, ?, ?
) RETURNING *;

-- name: GetNetworkNode :one
SELECT * FROM network_nodes
WHERE network_id = ? AND node_id = ?;

-- name: ListNetworkNodesByNetwork :many
SELECT * FROM network_nodes
WHERE network_id = ?
ORDER BY created_at DESC;

-- name: ListNetworkNodesByNode :many
SELECT * FROM network_nodes
WHERE node_id = ?
ORDER BY created_at DESC;

-- name: UpdateNetworkNodeStatus :one
UPDATE network_nodes
SET status = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE network_id = ? AND node_id = ?
RETURNING *;

-- name: UpdateNetworkNodeRole :one
UPDATE network_nodes
SET role = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE network_id = ? AND node_id = ?
RETURNING *;

-- name: DeleteNetworkNode :exec
DELETE FROM network_nodes
WHERE network_id = ? AND node_id = ?;

-- name: GetNetworkNodes :many
SELECT nn.*, n.* 
FROM network_nodes nn
JOIN nodes n ON nn.node_id = n.id
WHERE nn.network_id = ? 
ORDER BY nn.created_at DESC;


-- name: CheckNetworkNodeExists :one
SELECT EXISTS(SELECT 1 FROM network_nodes WHERE network_id = ? AND node_id = ?);

-- name: UpdateNetworkCurrentConfigBlock :exec
UPDATE networks
SET current_config_block_b64 = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: GetNetworkCurrentConfigBlock :one
SELECT current_config_block_b64 FROM networks
WHERE id = ?;

-- name: GetKeyByEthereumAddress :one
SELECT k.*, kp.name as provider_name, kp.type as provider_type
FROM keys k
JOIN key_providers kp ON k.provider_id = kp.id
WHERE k.ethereum_address = ?;

-- name: GetKeysByFilter :many
SELECT k.*, kp.name as provider_name, kp.type as provider_type
FROM keys k
JOIN key_providers kp ON k.provider_id = kp.id
WHERE (@algorithm_filter = '' OR k.algorithm = @algorithm) 
  AND (@provider_id_filter = 0 OR k.provider_id = @provider_id)
  AND (@curve_filter = '' OR k.curve = @curve);

-- name: UpdateNodeEndpoint :one
UPDATE nodes
SET endpoint = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateNodePublicEndpoint :one
UPDATE nodes
SET public_endpoint = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: GetPeerPorts :many
SELECT endpoint, public_endpoint
FROM nodes
WHERE node_type = 'fabric-peer'
AND (endpoint IS NOT NULL OR public_endpoint IS NOT NULL);

-- name: GetOrdererPorts :many
SELECT endpoint, public_endpoint
FROM nodes
WHERE node_type = 'fabric-orderer'
AND (endpoint IS NOT NULL OR public_endpoint IS NOT NULL);

-- name: CreateBackupTarget :one
INSERT INTO backup_targets (
    name,
    type,
    bucket_name,
    region,
    endpoint,
    bucket_path,
    access_key_id,
    secret_key,
    s3_path_style,
    restic_password,
    created_at,
    updated_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetBackupTarget :one
SELECT * FROM backup_targets
WHERE id = ? LIMIT 1;

-- name: ListBackupTargets :many
SELECT * FROM backup_targets
ORDER BY created_at DESC;

-- name: DeleteBackupTarget :exec
DELETE FROM backup_targets WHERE id = ?;

-- name: CreateBackupSchedule :one
INSERT INTO backup_schedules (
    name,
    description,
    cron_expression,
    target_id,
    retention_days,
    enabled,
    created_at,
    updated_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListBackupSchedules :many
SELECT * FROM backup_schedules
ORDER BY created_at DESC;

-- name: GetBackupSchedule :one
SELECT * FROM backup_schedules
WHERE id = ? LIMIT 1;

-- name: UpdateBackupScheduleLastRun :one
UPDATE backup_schedules
SET last_run_at = ?,
    next_run_at = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: EnableBackupSchedule :one
UPDATE backup_schedules
SET enabled = true,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DisableBackupSchedule :one
UPDATE backup_schedules
SET enabled = false,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteBackupSchedule :exec
DELETE FROM backup_schedules WHERE id = ?;

-- name: CreateBackup :one
INSERT INTO backups (
    schedule_id,
    target_id,
    status,
    started_at,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    CURRENT_TIMESTAMP
) RETURNING *;

-- name: UpdateBackupStatus :one
UPDATE backups
SET status = ?
WHERE id = ?
RETURNING *;

-- name: UpdateBackupCompleted :one
UPDATE backups
SET status = ?,
    completed_at = ?
WHERE id = ?
RETURNING *;

-- name: UpdateBackupFailed :one
UPDATE backups
SET status = ?,
    error_message = ?,
    completed_at = ?
WHERE id = ?
RETURNING *;

-- name: ListBackups :many
SELECT * FROM backups
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetBackup :one
SELECT * FROM backups
WHERE id = ? LIMIT 1;

-- name: DeleteBackup :exec
DELETE FROM backups WHERE id = ?;

-- name: ListBackupsBySchedule :many
SELECT * FROM backups
WHERE schedule_id = ?
ORDER BY created_at DESC;

-- name: ListBackupsByTarget :many
SELECT * FROM backups
WHERE target_id = ?
ORDER BY created_at DESC;

-- name: UpdateBackupSize :one
UPDATE backups
SET size_bytes = ?
WHERE id = ?
RETURNING *;

-- name: GetBackupsByStatus :many
SELECT * FROM backups
WHERE status = ?
ORDER BY created_at DESC;

-- name: GetBackupsByDateRange :many
SELECT * FROM backups
WHERE created_at BETWEEN ? AND ?
ORDER BY created_at DESC;

-- name: GetBackupsByScheduleAndStatus :many
SELECT * FROM backups
WHERE schedule_id = ? AND status = ?
ORDER BY created_at DESC;

-- name: CountBackupsByTarget :one
SELECT COUNT(*) FROM backups
WHERE target_id = ?;

-- name: CountBackupsBySchedule :one
SELECT COUNT(*) FROM backups
WHERE schedule_id = ?;

-- name: GetOldestBackupByTarget :one
SELECT * FROM backups
WHERE target_id = ?
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteBackupsBySchedule :exec
DELETE FROM backups
WHERE schedule_id = ?;

-- name: DeleteBackupsByTarget :exec
DELETE FROM backups
WHERE target_id = ?;

-- name: DeleteOldBackups :exec
DELETE FROM backups
WHERE target_id = ? 
AND created_at < ?;

-- name: UpdateBackupTarget :one
UPDATE backup_targets
SET name = ?,
    type = ?,
    bucket_name = ?,
    region = ?,
    endpoint = ?,
    bucket_path = ?,
    access_key_id = ?,
    secret_key = ?,
    s3_path_style = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateBackupSchedule :one
UPDATE backup_schedules
SET name = ?,
    description = ?,
    cron_expression = ?,
    target_id = ?,
    retention_days = ?,
    enabled = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: CreateNotificationProvider :one
INSERT INTO notification_providers (
    type,
    name,
    config,
    is_default,
    notify_node_downtime,
    notify_backup_success,
    notify_backup_failure,
    notify_s3_connection_issue,
    created_at,
    updated_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) RETURNING *;

-- name: UpdateNotificationProvider :one
UPDATE notification_providers
SET type = ?,
    name = ?,
    config = ?,
    is_default = ?,
    notify_node_downtime = ?,
    notify_backup_success = ?,
    notify_backup_failure = ?,
    notify_s3_connection_issue = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: GetNotificationProvider :one
SELECT * FROM notification_providers
WHERE id = ? LIMIT 1;

-- name: ListNotificationProviders :many
SELECT * FROM notification_providers
ORDER BY created_at DESC;

-- name: DeleteNotificationProvider :exec
DELETE FROM notification_providers
WHERE id = ?;

-- name: GetDefaultNotificationProvider :one
SELECT * FROM notification_providers
WHERE is_default = 1 AND type = ?
LIMIT 1;

-- name: UnsetDefaultNotificationProvider :exec
UPDATE notification_providers
SET is_default = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE type = ? AND is_default = 1;

-- name: UpdateProviderTestResults :one
UPDATE notification_providers
SET last_test_at = ?,
    last_test_status = ?,
    last_test_message = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: GetProvidersByNotificationType :many
SELECT * FROM notification_providers
WHERE (
    (? = 'NODE_DOWNTIME' AND notify_node_downtime = 1) OR
    (? = 'BACKUP_SUCCESS' AND notify_backup_success = 1) OR
    (? = 'BACKUP_FAILURE' AND notify_backup_failure = 1) OR
    (? = 'S3_CONNECTION_ISSUE' AND notify_s3_connection_issue = 1)
)
ORDER BY created_at DESC;

-- name: GetRecentCompletedBackups :many
SELECT * FROM backups
WHERE (status = 'COMPLETED' OR status = 'FAILED')
  AND notification_sent = false
ORDER BY completed_at DESC
LIMIT 50;

-- name: MarkBackupNotified :exec
UPDATE backups
SET notification_sent = true
WHERE id = ?;

-- name: GetDefaultNotificationProviderForType :one
SELECT * FROM notification_providers
WHERE is_default = true
  AND (
    (:notification_type = 'BACKUP_SUCCESS' AND notify_backup_success = true) OR
    (:notification_type = 'BACKUP_FAILURE' AND notify_backup_failure = true) OR
    (:notification_type = 'NODE_DOWNTIME' AND notify_node_downtime = true) OR
    (:notification_type = 'S3_CONNECTION_ISSUE' AND notify_s3_connection_issue = true)
  )
LIMIT 1;

-- name: AddRevokedCertificate :exec
INSERT INTO fabric_revoked_certificates (
    fabric_organization_id,
    serial_number,
    revocation_time,
    reason,
    issuer_certificate_id
) VALUES (?, ?, ?, ?, ?);

-- name: GetRevokedCertificates :many
SELECT * FROM fabric_revoked_certificates
WHERE fabric_organization_id = ?
ORDER BY revocation_time DESC;

-- name: GetRevokedCertificate :one
SELECT * FROM fabric_revoked_certificates
WHERE fabric_organization_id = ? AND serial_number = ?;

-- name: UpdateOrganizationCRL :exec
UPDATE fabric_organizations
SET crl_last_update = ?,
    crl_key_id = ?
WHERE id = ?;

-- name: GetOrganizationCRLInfo :one
SELECT crl_key_id, crl_last_update
FROM fabric_organizations
WHERE id = ?;

-- name: CreateSetting :one
INSERT INTO settings (
    config
) VALUES (
    ?
)
RETURNING *;

-- name: GetSetting :one
SELECT * FROM settings
WHERE id = ? LIMIT 1;

-- name: ListSettings :many
SELECT * FROM settings
ORDER BY created_at DESC;

-- name: UpdateSetting :one
UPDATE settings
SET config = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteSetting :exec
DELETE FROM settings
WHERE id = ?; 

-- name: DeleteRevokedCertificate :exec
DELETE FROM fabric_revoked_certificates
WHERE fabric_organization_id = ? AND serial_number = ?;

-- name: GetRevokedCertificateCount :one
SELECT COUNT(*) FROM fabric_revoked_certificates
WHERE fabric_organization_id = ?;

-- name: CreatePlugin :one
INSERT INTO plugins (
  name,
  api_version,
  kind,
  metadata,
  spec,
  created_at,
  updated_at
) VALUES (
  ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: UpdatePlugin :one
UPDATE plugins
SET 
  api_version = ?,
  kind = ?,
  metadata = ?,
  spec = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE name = ?
RETURNING *;


-- name: GetPlugin :one
SELECT * FROM plugins WHERE name = ?;

-- name: ListPlugins :many
SELECT * FROM plugins ORDER BY name;

-- name: DeletePlugin :exec
DELETE FROM plugins WHERE name = ?;


-- name: UpdateDeploymentMetadata :exec
UPDATE plugins
SET deployment_metadata = ?
WHERE name = ?;

-- name: UpdateDeploymentStatus :exec
UPDATE plugins
SET deployment_status = ?
WHERE name = ?;

-- name: GetDeploymentMetadata :one
SELECT deployment_metadata
FROM plugins
WHERE name = ?;

-- name: GetDeploymentStatus :one
SELECT deployment_status
FROM plugins
WHERE name = ?; 


-- name: GetSessionBySessionID :one
SELECT * FROM sessions
WHERE session_id = ?;

-- name: GetSessionByToken :one
SELECT * FROM sessions
WHERE token = ?;

-- name: GetPrometheusConfig :one
SELECT * FROM prometheus_config
WHERE id = 1;

-- name: UpdatePrometheusConfig :one
UPDATE prometheus_config
SET prometheus_port = ?,
    data_dir = ?,
    config_dir = ?,
    container_name = ?,
    scrape_interval = ?,
    evaluation_interval = ?,
    deployment_mode = ?,
    docker_image = ?,
    docker_network = ?,
    docker_restart_policy = ?,
    docker_extra_args = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1
RETURNING *;

-- name: ResetPrometheusConfig :one
UPDATE prometheus_config
SET prometheus_port = 9090,
    data_dir = '/var/lib/prometheus',
    config_dir = '/etc/prometheus',
    container_name = 'chainlaunch-prometheus',
    scrape_interval = 15,
    evaluation_interval = 15,
    deployment_mode = 'docker',
    docker_image = 'prom/prometheus:latest',
    docker_network = 'chainlaunch-network',
    docker_restart_policy = 'unless-stopped',
    docker_extra_args = '--web.enable-lifecycle --web.enable-admin-api',
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1
RETURNING *;

-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    timestamp,
    event_source,
    user_identity,
    source_ip,
    event_type,
    event_outcome,
    affected_resource,
    request_id,
    severity,
    details,
    session_id
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetAuditLog :one
SELECT * FROM audit_logs
WHERE id = ? LIMIT 1;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
WHERE (? IS NULL OR timestamp >= ?)
  AND (? IS NULL OR timestamp <= ?)
  AND (? = '' OR event_type = ?)
  AND (? = 0 OR user_identity = ?)
ORDER BY timestamp DESC
LIMIT ? OFFSET ?;

-- name: CountAuditLogs :one
SELECT COUNT(*) FROM audit_logs
WHERE (? IS NULL OR timestamp >= ?)
  AND (? IS NULL OR timestamp <= ?)
  AND (? = '' OR event_type = ?)
  AND (? = '' OR user_identity = ?);
