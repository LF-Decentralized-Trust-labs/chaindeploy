# ADR 0001: Node Groups and Managed Services

- **Status:** Proposed
- **Date:** 2026-04-16
- **Deciders:** David Viejo
- **Scope:** `chaindeploy/`

## Context

FabricX introduces two node kinds that, in the current model, each collapse multiple independently-running containers into a single `nodes` row:

- **FabricX orderer group** — 4 containers: `router`, `batcher`, `consenter`, `assembler`.
- **FabricX committer** — 5 containers: `sidecar`, `coordinator`, `validator`, `verifier`, `query-service`; plus a supporting **PostgreSQL** container holding committed block state.

Today this is modelled as one `nodes` row per group, with the roles as sibling fields on the same deployment config. This breaks the platform's core invariant — **one `nodes` row = one runnable unit** — and causes concrete problems:

1. **No per-role lifecycle.** `start`/`stop`/`restart`/`delete` only exists at group granularity. An operator cannot restart just the `batcher` to pick up a config change.
2. **No per-role observability.** Logs, events, metrics, and health are aggregated across roles. Debugging a stuck `consenter` requires grepping combined output.
3. **Status is a lie.** A group row reports `RUNNING` even when 1 of 4 containers is down.
4. **No supporting-infra model.** The committer's PostgreSQL is a production data store (it holds the committed ledger) but is currently spun up as a hidden side-effect of `Committer.Start()`. It has no row, no status, no backup, no retention policy, no restore path. Losing it means replaying from the ordering service, which may not be feasible depending on retention.
5. **UI dead-end.** The detail view (see earlier session) can either show a single pile of fields or special-case every group type.

## Decision

Introduce two new first-class concepts in `chaindeploy`:

### 1. `node_groups` — a new table

A `node_group` is a **logical parent** that owns shared identity/crypto material and orchestrates a set of child `nodes`. The group itself is **not a runnable container** — it is metadata plus a lifecycle coordinator.

Rationale for a new table (vs. `nodes.parent_node_id` self-reference):

- Groups and nodes have genuinely different semantics (group has no container, no endpoint; node does). Polymorphic self-reference leaks that difference into every query.
- Groups carry shared state (MSP, certs, domain names) that would be duplicated or forced-null across children in a self-reference model.
- A separate table makes listing / filtering / permissions / auditing cleaner.

### 2. `services` — a new table for supporting infrastructure

A `service` is a **managed supporting component** attached to a `node_group` (typically) or standalone. PostgreSQL is the first citizen; Redis/Kafka/sidecars can follow.

Rationale for a separate `services` table (vs. adding a `POSTGRES` node type):

- Services are not blockchain nodes and should not appear in `/nodes` lists or pollute `nodes.platform` / `nodes.node_type` enums.
- Services have their own operational concerns (backups, restores, PITR) that don't apply to blockchain nodes.
- Keeping the boundary clean means "show me all my peers" stays a simple query, and future services (key-vault agents, metric sidecars) don't distort the blockchain inventory.

### 3. Child node types for FabricX

New `TypesNodeType` values, one per container role:

- Orderer group children: `FABRICX_ORDERER_ROUTER`, `FABRICX_ORDERER_BATCHER`, `FABRICX_ORDERER_CONSENTER`, `FABRICX_ORDERER_ASSEMBLER`
- Committer children: `FABRICX_COMMITTER_SIDECAR`, `FABRICX_COMMITTER_COORDINATOR`, `FABRICX_COMMITTER_VALIDATOR`, `FABRICX_COMMITTER_VERIFIER`, `FABRICX_COMMITTER_QUERY_SERVICE`

The existing `FABRICX_ORDERER_GROUP` and `FABRICX_COMMITTER` values become `node_groups.group_type` — they leave the `nodes.node_type` enum. (See Migration below for how legacy rows are handled.)

### 4. PostgreSQL as a managed `service` with WAL-G

The committer's PostgreSQL becomes a `services` row of type `POSTGRES`, with:

- **Main container:** `postgres:16-alpine` (pinned — must match fabric-x expectations).
- **WAL-G sidecar container:** sharing the postgres data volume. `archive_command` in `postgresql.conf` is set to `wal-g wal-push %p`, so every WAL segment is streamed to S3 as postgres writes it.
- **Daily base backup:** scheduled at the ChainLaunch level (not inside the sidecar) — runs `wal-g backup-push /var/lib/postgresql/data` via `docker exec` into the sidecar. Retries, failures, and success events land in the same `service_events` feed as other operations.
- **Retention:** configurable per-service (default: 7 base backups + 30 days of WAL).
- **S3 credentials:** each `services` row references an existing `backup_targets(id)` — we reuse that table's encrypted S3 auth. No second credential store.
- **Restore:** `POST /services/{id}/restore` with `{backup_id, pitr_timestamp?}`. Stops the postgres container, wipes the data volume, runs `wal-g backup-fetch` + optional `recovery_target_time`, restarts. The committer node group must be stopped first (enforced by handler).

### 5. Lifecycle orchestration

- **Group start**: creates/starts the required `services` first (postgres), waits for readiness, then starts children in dependency order.
- **Group stop**: stops children in reverse order, optionally stops services (configurable — default keeps postgres running to avoid connection storms on restart).
- **Per-child operations**: `POST /nodes/{id}/{start,stop,restart}` works on any child without touching siblings.
- **Group status** = derived from children's statuses: `RUNNING` only if all children RUNNING; `DEGRADED` if some; `STOPPED` if all stopped; `ERROR` if any errored.

## Schema

```sql
-- Migration 0022 (up)

CREATE TABLE node_groups (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL UNIQUE,
    platform           TEXT NOT NULL,                -- FABRICX
    group_type         TEXT NOT NULL,                -- FABRICX_ORDERER_GROUP | FABRICX_COMMITTER
    msp_id             TEXT,
    organization_id    INTEGER REFERENCES fabric_organizations(id),
    party_id           INTEGER,
    version            TEXT,
    external_ip        TEXT,
    domain_names       TEXT,                         -- JSON array
    sign_key_id        INTEGER,
    tls_key_id         INTEGER,
    sign_cert          TEXT,
    tls_cert           TEXT,
    ca_cert            TEXT,
    tls_ca_cert        TEXT,
    config             TEXT,                         -- JSON, group-shared
    deployment_config  TEXT,                         -- JSON, group-shared (network name, shared mounts)
    status             TEXT NOT NULL DEFAULT 'CREATED',
    error_message      TEXT,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP
);

CREATE INDEX idx_node_groups_platform ON node_groups(platform);
CREATE INDEX idx_node_groups_group_type ON node_groups(group_type);
CREATE INDEX idx_node_groups_status ON node_groups(status);

-- Child nodes gain a nullable foreign key to their group.
ALTER TABLE nodes ADD COLUMN node_group_id INTEGER REFERENCES node_groups(id) ON DELETE CASCADE;
CREATE INDEX idx_nodes_node_group_id ON nodes(node_group_id);

-- Services
CREATE TABLE services (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    node_group_id      INTEGER REFERENCES node_groups(id) ON DELETE CASCADE,
    name               TEXT NOT NULL UNIQUE,
    service_type       TEXT NOT NULL,                -- POSTGRES, REDIS, ...
    version            TEXT,
    status             TEXT NOT NULL DEFAULT 'CREATED',
    config             TEXT,                         -- JSON (per-type, e.g. pg credentials, port, db name)
    deployment_config  TEXT,                         -- JSON (container names, ports, volumes)
    backup_target_id   INTEGER REFERENCES backup_targets(id),
    backup_config      TEXT,                         -- JSON (schedule, retention, wal-g prefix)
    error_message      TEXT,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP
);

CREATE INDEX idx_services_node_group_id ON services(node_group_id);
CREATE INDEX idx_services_service_type ON services(service_type);
CREATE INDEX idx_services_status ON services(status);

CREATE TABLE service_backups (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id         INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    backup_type        TEXT NOT NULL,                -- BASE, WAL
    s3_key             TEXT,                         -- WAL-G label or base backup name
    size_bytes         INTEGER,
    lsn                TEXT,                         -- postgres-specific
    timeline           INTEGER,                      -- postgres-specific
    status             TEXT NOT NULL,                -- PENDING, IN_PROGRESS, COMPLETED, FAILED
    started_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at       TIMESTAMP,
    error_message      TEXT,
    metadata           TEXT                          -- JSON, type-specific
);

CREATE INDEX idx_service_backups_service_id ON service_backups(service_id);
CREATE INDEX idx_service_backups_status ON service_backups(status);

CREATE TABLE service_events (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id         INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    type               TEXT NOT NULL,                -- START, STOP, BACKUP, RESTORE, ERROR
    status             TEXT NOT NULL,
    data               TEXT,                         -- JSON
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_service_events_service_id ON service_events(service_id);
```

Legacy `FABRICX_ORDERER_GROUP` / `FABRICX_COMMITTER` rows in `nodes` are migrated in a **separate script**, not in the SQL migration (see Plan, Phase 5). The migration inserts the new type enum rows; it does not delete the old ones until Phase 6.

## API surface

### Node groups

- `GET    /api/v1/node-groups` — list, paginated
- `POST   /api/v1/node-groups` — create (platform + group_type + shared config); server provisions children + services
- `GET    /api/v1/node-groups/{id}` — full detail: group fields + child nodes + attached services
- `DELETE /api/v1/node-groups/{id}` — cascade-delete children + services
- `POST   /api/v1/node-groups/{id}/start` — cascade-start
- `POST   /api/v1/node-groups/{id}/stop` — cascade-stop
- `POST   /api/v1/node-groups/{id}/restart`

### Services

- `GET    /api/v1/services` — list, optionally filtered by `?node_group_id=`
- `GET    /api/v1/services/{id}`
- `POST   /api/v1/services/{id}/start` / `stop` / `restart`
- `DELETE /api/v1/services/{id}` — refuses if attached group is running
- `GET    /api/v1/services/{id}/backups` — backup history
- `POST   /api/v1/services/{id}/backups` — trigger manual base backup
- `POST   /api/v1/services/{id}/restore` — body `{backup_id, pitr_timestamp?}`

Existing `/api/v1/nodes/{id}` continues to work for child nodes — a child's response includes a `nodeGroupId` pointer so the frontend can render a link back to the parent.

## Consequences

### Positive

- **Restores the "1 row = 1 unit" invariant.** Every `nodes` row is exactly one container again; logs/metrics/events are per-role.
- **Operational granularity.** Ops can restart a single batcher without disturbing peers.
- **Honest status.** Group status reflects reality; degraded states are visible.
- **Production-grade postgres.** Continuous archiving + PITR means committer state survives container loss, host failure, and operator error.
- **Reusable services primitive.** Future supporting infra (metrics sidecars, vault agents) lands in the same model.
- **Clean UI.** The detail page is a tree: group summary → children list → per-child detail → services list → per-service detail + backup history.

### Negative

- **Schema churn.** One migration + one data-migration script. SQLite migrations are usually additive, so the risk is low, but the data-migration needs care.
- **API surface grows.** Three new resource families (`node-groups`, `services`, `service-backups`). Handlers, sqlc queries, OpenAPI annotations, TypeScript regeneration.
- **FabricX package rewrite.** `pkg/nodes/fabricx/orderergroup.go` and `committer.go` get their loops unrolled into per-child `Init/Start/Stop` units. Non-trivial, but mechanical.
- **WAL-G adds an image dependency** and a new failure mode (S3 unreachable → archiving backs up → postgres disk fills). Mitigated with monitoring + `archive_timeout` + alerts.

### Risks & mitigations

| Risk | Mitigation |
|---|---|
| Data migration drops or mangles existing FabricX groups | Script runs in a transaction; dry-run mode prints the plan; taken + committed only on confirm. DB backed up first. |
| Children started out of order (e.g. consenter before batcher) | Explicit start-order in service layer; per-role readiness checks using existing health probes. |
| WAL-G archive failures cause postgres to block | Set `archive_command` with retries; alert via existing notification system when backup events fail. |
| Frontend regressions during the transition | Phase 3 ships behind a feature flag; old monolithic detail view stays wired until Phase 4 is verified. |
| Group deletion leaves orphan containers | `DELETE` handler tears down children + services before removing rows; transactional in DB, best-effort in docker with reconciler sweep. |

## Alternatives considered

1. **`nodes.parent_node_id` self-reference.** Rejected: polymorphism forces null-padding for "this isn't a real container" rows and complicates every query (`WHERE parent_node_id IS NULL AND ...`).
2. **PostgreSQL as a node type.** Rejected: pollutes the blockchain-node inventory; operational concerns (backup, restore) don't fit the node lifecycle.
3. **Embedded WAL-G in a custom postgres image.** Rejected: harder to upgrade WAL-G independently, and coupling postgres version bumps to backup-tool bumps is bad hygiene.
4. **Reuse `backup_targets` + `backups` + `backup_schedules` tables directly for postgres.** Rejected for the backup *record*: WAL streaming produces thousands of WAL segments per day, which blow up the `backups` table semantically. Reuse `backup_targets` for credentials only.
5. **Defer services entirely, ship node_groups first.** Considered. Rejected because committer postgres is already in prod without a backup path — shipping the group refactor without a durable data story leaves the door open for data loss.

## Out of scope (follow-ups)

- HA postgres (streaming replica). Current design is single-writer. A follow-up ADR covers `services.replica_of` if demand arises.
- Kubernetes deployment target. Current design assumes docker runtime. Kubernetes comes as a separate orchestrator behind the same `node_groups` / `services` API.
- Chainlaunch-pro RBAC for groups/services. Pro adds per-resource permissions; this ADR leaves that to pro's existing middleware.
- Export/import of groups in network templates. Network template v2.0.0 variable system (see memory: 2026-03-20) needs a follow-up to emit group-aware templates.
