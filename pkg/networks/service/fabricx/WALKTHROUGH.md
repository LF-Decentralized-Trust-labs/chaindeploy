# FabricX network walkthrough

A hands-on reflection on deploying a 4-party FabricX network on macOS/Docker
Desktop via ChainLaunch, and what worked / what didn't. Written after
actually doing it end-to-end, not from the README.

## The 10,000-ft flow

There are three deliverables, each exposed as REST endpoints:

1. **Orderer groups** (one per party) — a 4-container unit: router, batcher,
   consenter, assembler. Created via `POST /api/v1/nodes` with
   `nodeType=FABRICX_ORDERER_GROUP`. `Init()` provisions keys, writes MSP/TLS,
   renders configs.
2. **Committers** (one per party) — a 6-container unit: sidecar, coordinator,
   validator, verifier, query-service, postgres. Created via `POST /api/v1/nodes`
   with `nodeType=FABRICX_COMMITTER`.
3. **Network** — `POST /api/v1/networks/fabricx` takes the list of orgs and
   (orderer, committer) pairings, generates an arma genesis block via
   `fabric-x-common/configtxgen`, and **stores the genesis on the network
   row only**. The orderer groups and committers do not yet know about it.
4. **Join (per node)** — `POST .../networks/fabricx/{id}/nodes/{nid}/join` is
   the step that pushes the genesis block out:
   - `FabricXDeployer.JoinNode` (`deployer.go:173`) fetches the stored
     genesis for the network.
   - Calls `nodeService.SetFabricXGenesisBlock` which writes
     `genesis/genesis.block` into every component's bind-mounted dir
     (`OrdererGroup.SetGenesisBlock` writes to router/batcher/consenter/
     assembler; `Committer.SetGenesisBlock` writes to sidecar).
   - Then calls `StartNode`, which boots the containers with the freshly
     written genesis.
   This is why the join loop matters — **creating the network alone does
   nothing observable**. Without the join calls, the orderer groups sit
   idle (or started against an old genesis from a previous run, which is
   how the `ABORTED_SIGNATURE_INVALID` class of bugs sneaks in).
5. **Namespace** — `POST .../{id}/namespaces` broadcasts a signed
   applicationpb.Tx to the router and (optionally) waits for committer
   finality.

## The happy path (assuming nothing is wrong)

```bash
# Required env var for macOS/Windows Docker Desktop
export CHAINLAUNCH_FABRICX_LOCAL_DEV=true

# 1. Create 4 orgs, each with its own sign + TLS CA (one-time, per org)
curl -X POST /api/v1/organizations -d '{"mspId":"Party1MSP",...}'

# 2. Create 4 orderer groups
for p in 1 2 3 4; do
  curl -X POST /api/v1/nodes -d @orderer-party$p.json
done

# 3. Create 4 committers
for p in 1 2 3 4; do
  curl -X POST /api/v1/nodes -d @committer-party$p.json
done

# 4. Create the network (generates genesis from all 4 orgs)
curl -X POST /api/v1/networks/fabricx -d '{"name":"...","organizations":[...]}'

# 5. Join every node to the network (writes genesis, starts containers)
for nid in 165 166 167 168 169 170 171 172; do
  curl -X POST /api/v1/networks/fabricx/22/nodes/$nid/join
done

# 6. Create a namespace
curl -X POST /api/v1/networks/fabricx/22/namespaces \
  -d '{"name":"token","submitterOrgId":30,"waitForFinality":true}'
```

On a clean machine with a warm Docker Desktop, this should work. In practice,
step 5 was where most of the friction lived.

## How easy was it, honestly?

**Once the network is stable: very easy.** The `/namespaces` endpoint is a
single POST, returns `{status:"committed", txId:"..."}` in ~3 seconds.
Subsequent namespaces ("token", "mycc_fresh5") both succeed in one shot.

**Getting to a stable network on macOS: rough.** I hit five distinct issues
on the way. None are fundamental to FabricX — all are operational friction
that a platform product should absorb.

## What actually went wrong (and how to avoid it)

### 1. Docker Desktop hairpin NAT

**Symptom:** Router containers can't reach each other through the configured
LAN IP (`192.168.1.133`); the namespace broadcast also can't dial the router
through that IP from the host.

**Fix in code:** `CHAINLAUNCH_FABRICX_LOCAL_DEV=true` triggers two swaps:
- For container-to-container traffic, `externalIP` → `host.docker.internal`
  in the genesis block, plus `--add-host externalIP:host-gateway` on every
  container.
- For host-to-container dials (the namespace broadcast), the configured
  external IP is replaced by `127.0.0.1`, and the gRPC TLS `ServerName` is
  pinned to `localhost` so the cert's SAN list still validates.

**Lesson:** the env var needs to be set on the `chainlaunch serve` process,
not just on containers. I forgot it on a restart and lost 10 minutes chasing
a "dial context deadline exceeded" that was really a loopback miss.

### 2. `FabricXNetworkDelete` doesn't clean containers or volumes

When I tore down a network to rebuild, the DB rows went away but the Docker
containers, bind-mounted data, and postgres volumes stayed. On the next run
the committers resumed from their old ledger position (requesting block 2
from a freshly-regenerated orderer at block 1), which surfaced as
`ABORTED_SIGNATURE_INVALID` on the namespace tx.

**Workaround:** before recreating a network, manually
```bash
docker ps -a --filter name=fabricx -q | xargs -r docker rm -f
rm -rf chaindeploy/data/fabricx-{orderers,committers}
docker volume prune -f
```

**Real fix (not done):** `FabricXNetworkDelete` and `NodeDelete` should purge
containers, bind mounts, and named volumes.

### 3. Wiping `data/` destroys keys that `Init()` wrote once

After step 2, I did the full wipe. But `Init()` runs only at node *creation*,
and it's what writes `msp/` and `tls/` under `data/fabricx-orderers/...`.
`Start()` previously only re-rendered configs. So a wipe + restart left every
component without a TLS keypair, and Docker failed the bind mount before the
container even launched.

**Fix applied (this session):** added `ensureMaterials()` to both
`OrdererGroup.Start` (`pkg/nodes/fabricx/orderergroup.go:317`) and
`Committer.Start` (`pkg/nodes/fabricx/committer.go:372`). It checks whether
`tls/server.key` exists; if not, it fetches the sign+TLS private keys from
the DB using `SignKeyID`/`TLSKeyID` stored in the deployment config and
rewrites MSP + TLS from scratch. Identity is preserved across full wipes.

### 4. Docker Desktop bind-mount negative cache

Even after `ensureMaterials` wrote the files, `docker create` kept returning
`invalid mount config ... bind source path does not exist: /host_mnt/...`
for 30+ seconds per attempt. The chainlaunch deployer has a retry loop with
exponential backoff (`common.go:220`), but the container start timeout is
90 seconds, which wasn't enough under a cold Docker Desktop with 4 parties
starting at once.

**Workaround:** call the join endpoint with `--max-time 240` and retry each
node individually. After the first component launches, Docker's cache warms
up and the rest succeed.

**Real fix (not done):** the retry loop should be longer, or the deployer
should `touch` the dir and do a lightweight `docker run --rm alpine ls` warm-up
before the first real container create.

### 5. Stale `tlsCaCert` / `caCert` in node `deployment_config`

This was the deepest one. Each node's `deployment_config` JSON carries a
snapshot of the TLS CA and sign CA certs at the time `Init()` ran. But when
an org's CA keys get rotated (or when the org was partially recreated —
which happened to me), the `fabric_organizations.tls_root_key_id` points
to the new CA, but the old nodes' `deployment_config.tlsCaCert` still
references the previous CA's certificate. `ensureMaterials` rewrote
`tls/ca.crt` from the stale deployment_config, while the server cert (also
from deployment_config) had been issued by a *different* CA. Result: the
router presented a cert that didn't chain to its own `ca.crt`, and the
namespace broadcast (which uses `deployCfg.TLSCACert` as the root) couldn't
verify the TLS handshake.

Diagnosis required:
```bash
openssl x509 -in server.crt -text | grep "Authority Key"
openssl x509 -in ca.crt -text | grep "Subject Key"
# → they didn't match
```

**Fix applied (ad-hoc, this session):**
```sql
UPDATE nodes
SET deployment_config = json_set(deployment_config,
  '$.tlsCaCert', (SELECT certificate FROM keys WHERE id=<org.tls_root_key_id>),
  '$.caCert',    (SELECT certificate FROM keys WHERE id=<org.sign_key_id>))
WHERE platform='FABRICX' AND json_extract(deployment_config,'$.organizationId')=<org_id>;
```

**Real fix (not done):** either
- `ensureMaterials` should re-fetch the org's current CAs from DB instead of
  trusting the cached deployment_config copy, or
- a migration/verify step should flag drift between an org's current CA and
  the CA snapshot in each node's config.

## What the current platform does well

- **REST API is complete.** Everything above was driven from curl. No hidden
  CLI steps, no YAML editing.
- **Two-stage Init+Start** is the right split. Init generates identity; Start
  is idempotent. Adding `ensureMaterials` restored that idempotency for
  full-wipe recovery.
- **Genesis generation** via `fabric-x-common/configtxgen` with explicit
  `PartyConfig`, `SharedConfig`, and `ConsenterMapping` is clean
  (`pkg/nodes/fabricx/genesis.go`). Easy to trace what went into the block.
- **Namespace tx signing** via fabric-admin-sdk's SigningIdentity, with the
  MSP-based verifier on the committer side — no custom signature code to
  maintain.
- **Status tracking**: `waitForFinality=true` makes the API synchronous,
  which is the right default for a platform that wants to look like a CRUD
  API rather than an async event bus.

## Gaps worth closing

Ranked by how much pain each one caused me:

1. **Network/node delete should actually delete.** Purge containers, bind
   mounts, postgres volumes, and genesis files. Stale state is the #1
   footgun on a dev loop.
2. **`ensureMaterials` should re-fetch CA certs from the org**, not from the
   frozen deployment_config copy. Or surface an explicit "CA drift" error
   instead of an opaque TLS handshake failure.
3. **Docker Desktop warm-up step** before the first container create, so the
   first join doesn't eat a 90s backoff budget.
4. **A single `POST /api/v1/networks/fabricx/{id}/start` endpoint** that does
   the join loop server-side. The current per-node loop is fine for
   scripting, but every real user will paper it over with a bash for-loop
   — the platform should do that.
5. **Healthcheck polling** after join, returning only when consensus is
   established (leader elected, deliver stream alive). Right now, the API
   returns `joined` as soon as containers are up, and the first namespace
   request might hit a router that isn't ready yet.
6. **A UI flow** that wraps all of steps 1–6. The mechanics are sound; the
   experience needs packaging.

## Bottom line

For someone who already understands FabricX topology and has ChainLaunch
configured, the end-to-end flow is about an hour of work the first time
and a handful of curl calls thereafter. The *platform* does the hard parts
(genesis generation, key orchestration, container lifecycle, gRPC signing).
The *friction* is all in the corners: macOS Docker behavior, incomplete
deletes, and one CA-drift edge case.

None of the remaining gaps are architectural. They're all well-scoped
patches that would turn a 1-hour manual procedure into a 5-minute
self-service flow.
