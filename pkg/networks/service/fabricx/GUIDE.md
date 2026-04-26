# FabricX network guide

How to create a FabricX network with ChainLaunch from scratch, and how to run
**multiple networks side-by-side locally** by pinning each one to its own
port range.

This is the practical how-to. For the "what went wrong and why" narrative,
see `WALKTHROUGH.md` in this directory.

## Prerequisites

- ChainLaunch server running on `http://localhost:8100`
- Docker Desktop running (macOS/Windows) or Docker Engine (Linux)
- `curl` + `jq`
- Admin credentials (default: `admin` / `admin123`)

### Required env var for macOS / Windows Docker Desktop

```bash
export CHAINLAUNCH_FABRICX_LOCAL_DEV=true
```

This must be set on the `chainlaunch serve` process itself — not just in
your shell. It swaps:

- container-to-container addresses → `host.docker.internal`
- host-to-container dials → `127.0.0.1`
- gRPC TLS `ServerName` → `localhost` (so SANs still validate)

On Linux with native Docker you can leave it unset — containers can reach
each other by their real IP.

### Credentials helper

```bash
export CL="http://localhost:8100"
export AUTH="admin:admin123"
```

## Topology reference

A single FabricX network with N parties needs:

| Resource              | Count | Containers per unit              | Total (N=4) |
|-----------------------|-------|-----------------------------------|-------------|
| Organization + 2 CAs  | N     | —                                 | 4 orgs      |
| Orderer group         | N     | router, batcher, consenter, assembler | 16      |
| Committer             | N     | sidecar, coordinator, validator, verifier, query-service, postgres | 24 |
| **Total containers**  |       |                                   | **40**      |

Each orderer group and each committer exposes its own set of ports.
**Those ports are the variable you control to run multiple networks locally.**

## Port allocation strategy

### Default (single network): let ChainLaunch auto-allocate

If you omit port fields in the POST body (or send `0`), ChainLaunch picks
from its free-port pool. This is fine for one network.

### Multi-network: pin each network to a port band

Reserve a **100-port band per network**, so parties and components don't
collide. Example scheme for two networks, 4 parties each:

| Network  | Party | Router | Batcher | Consenter | Assembler | Sidecar | Coord. | Validator | Verifier | Query | Postgres |
|----------|-------|--------|---------|-----------|-----------|---------|--------|-----------|----------|-------|----------|
| **A**    | 1     | 17010  | 17011   | 17012     | 17013     | 17020   | 17021  | 17022     | 17023    | 17024 | 17025    |
| A        | 2     | 17030  | 17031   | 17032     | 17033     | 17040   | 17041  | 17042     | 17043    | 17044 | 17045    |
| A        | 3     | 17050  | 17051   | 17052     | 17053     | 17060   | 17061  | 17062     | 17063    | 17064 | 17065    |
| A        | 4     | 17070  | 17071   | 17072     | 17073     | 17080   | 17081  | 17082     | 17083    | 17084 | 17085    |
| **B**    | 1     | 17110  | 17111   | 17112     | 17113     | 17120   | 17121  | 17122     | 17123    | 17124 | 17125    |
| B        | 2     | 17130  | 17131   | 17132     | 17133     | 17140   | 17141  | 17142     | 17143    | 17144 | 17145    |
| B        | 3     | 17150  | 17151   | 17152     | 17153     | 17160   | 17161  | 17162     | 17163    | 17164 | 17165    |
| B        | 4     | 17170  | 17171   | 17172     | 17173     | 17180   | 17181  | 17182     | 17183    | 17184 | 17185    |

Rule of thumb:
- 100-port band per network: `17000+100*netIdx` .. `17099+100*netIdx`
- 20-port slot per party within the band: `band + 20*(partyIdx-1)`
- First 10 ports of a slot → orderer group; next 10 → committer

Pick any scheme you like — the only hard requirement is **no two
components share a port on the host**.

### Organizations are reusable across networks

Nothing forces you to create new orgs per network. If `Party1MSP` is
already a valid org, you can use it in multiple networks. The orderer
group and committer nodes are per-network, but orgs and their CAs are
not. In this guide we reuse orgs across networks A and B.

## Step 1 — Create organizations (once per org)

Each org gets a signing CA and a TLS CA. ChainLaunch generates both.

```bash
for p in 1 2 3 4; do
  curl -s -u "$AUTH" -X POST "$CL/api/v1/organizations" \
    -H "Content-Type: application/json" \
    -d "{
      \"mspId\": \"Party${p}MSP\",
      \"description\": \"FabricX Party ${p}\",
      \"providerId\": 1
    }" | jq '.id, .mspId'
done
```

Save the returned `id` for each org — you'll pass them into orderer
group and committer creation. Below we use `$ORG1`..`$ORG4`.

```bash
# Example lookup:
export ORG1=$(curl -s -u $AUTH "$CL/api/v1/organizations" | jq '.items[] | select(.mspId=="Party1MSP") | .id')
# (repeat for ORG2..ORG4)
```

## Step 2 — Create orderer groups

One orderer group per party. This is the 4-container unit
(router/batcher/consenter/assembler).

### Network A, Party 1

```bash
curl -s -u "$AUTH" -X POST "$CL/api/v1/nodes" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"netA-orderer-p1\",
    \"nodeType\": \"FABRICX_ORDERER_GROUP\",
    \"fabricxOrdererGroup\": {
      \"name\": \"netA-orderer-p1\",
      \"organizationId\": $ORG1,
      \"mspId\": \"Party1MSP\",
      \"partyId\": 1,
      \"externalIp\": \"127.0.0.1\",
      \"version\": \"latest\",
      \"consenterType\": \"pbft\",
      \"routerPort\": 17010,
      \"batcherPort\": 17011,
      \"consenterPort\": 17012,
      \"assemblerPort\": 17013
    }
  }"
```

Repeat for parties 2, 3, 4 with their ports from the table above.

**Validation rules:**
- `partyId` must be between 1 and 10
- `mspId` must match the organization's MSP ID
- `consenterType`: `"pbft"` (default) or `"raft"`

## Step 3 — Create committers

One committer per party. 6-container unit
(sidecar/coordinator/validator/verifier/query-service/postgres).

The `ordererEndpoints` field tells the sidecar which assembler(s) to pull
blocks from. In local-dev you can point at any one assembler
(`host.docker.internal:<assemblerPort>`), or all of them for redundancy.

### Network A, Party 1

```bash
curl -s -u "$AUTH" -X POST "$CL/api/v1/nodes" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"netA-committer-p1\",
    \"nodeType\": \"FABRICX_COMMITTER\",
    \"fabricxCommitter\": {
      \"name\": \"netA-committer-p1\",
      \"organizationId\": $ORG1,
      \"mspId\": \"Party1MSP\",
      \"externalIp\": \"127.0.0.1\",
      \"version\": \"latest\",
      \"sidecarPort\":      17020,
      \"coordinatorPort\":  17021,
      \"validatorPort\":    17022,
      \"verifierPort\":     17023,
      \"queryServicePort\": 17024,
      \"postgresPort\":     17025,
      \"postgresHost\":     \"host.docker.internal\",
      \"postgresDb\":       \"netA_p1\",
      \"postgresUser\":     \"fabricx\",
      \"postgresPassword\": \"fabricx\",
      \"channelId\":        \"arma\",
      \"ordererEndpoints\": [
        \"host.docker.internal:17013\",
        \"host.docker.internal:17033\",
        \"host.docker.internal:17053\",
        \"host.docker.internal:17073\"
      ]
    }
  }"
```

**Notes:**
- `postgresHost: "host.docker.internal"` + a distinct `postgresPort` per
  committer gives each party its own postgres container.
- `postgresDb` must be unique per committer if they share a postgres
  instance (they don't here — each gets its own).
- `ordererEndpoints` lists **assembler** ports, not router ports.

## Step 4 — Create the network

Generates the genesis block from the listed orgs and stores it on the
network row. No containers start at this step.

```bash
curl -s -u "$AUTH" -X POST "$CL/api/v1/networks/fabricx" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"netA\",
    \"description\": \"FabricX network A (ports 17010-17099)\",
    \"channelId\": \"arma\",
    \"consenterType\": \"pbft\",
    \"organizations\": [
      {\"organizationId\": $ORG1, \"mspId\": \"Party1MSP\", \"partyId\": 1},
      {\"organizationId\": $ORG2, \"mspId\": \"Party2MSP\", \"partyId\": 2},
      {\"organizationId\": $ORG3, \"mspId\": \"Party3MSP\", \"partyId\": 3},
      {\"organizationId\": $ORG4, \"mspId\": \"Party4MSP\", \"partyId\": 4}
    ],
    \"nodes\": [
      {\"partyId\": 1, \"ordererGroupName\": \"netA-orderer-p1\", \"committerName\": \"netA-committer-p1\"},
      {\"partyId\": 2, \"ordererGroupName\": \"netA-orderer-p2\", \"committerName\": \"netA-committer-p2\"},
      {\"partyId\": 3, \"ordererGroupName\": \"netA-orderer-p3\", \"committerName\": \"netA-committer-p3\"},
      {\"partyId\": 4, \"ordererGroupName\": \"netA-orderer-p4\", \"committerName\": \"netA-committer-p4\"}
    ]
  }" | jq '.id'
```

Capture the returned network ID as `$NETA_ID`.

## Step 5 — Join every node to the network

This is the step that **actually starts the containers**. It writes the
genesis block into each component's bind mount, then calls `StartNode`.

```bash
# Collect node IDs for netA
NODE_IDS=$(curl -s -u "$AUTH" "$CL/api/v1/nodes?platform=FABRICX" \
  | jq -r '.items[] | select(.name | startswith("netA-")) | .id')

for nid in $NODE_IDS; do
  echo "Joining node $nid..."
  curl -s -u "$AUTH" --max-time 240 \
    -X POST "$CL/api/v1/networks/fabricx/$NETA_ID/nodes/$nid/join" \
    | jq '.status'
done
```

**Why `--max-time 240`:** on macOS Docker Desktop the first container
start under a cold bind-mount cache can take 60–120 seconds. After one
component warms the cache, the rest succeed quickly. If a join times
out, retry it individually.

Verify all 8 nodes are running:

```bash
curl -s -u "$AUTH" "$CL/api/v1/nodes?platform=FABRICX" \
  | jq '.items[] | {id, name, status}'
```

All should show `"status": "RUNNING"`.

## Step 6 — Create a namespace

```bash
curl -s -u "$AUTH" -X POST "$CL/api/v1/networks/fabricx/$NETA_ID/namespaces" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"token\",
    \"submitterOrgId\": $ORG1,
    \"waitForFinality\": true
  }"
```

Response:
```json
{"id": 17, "status": "committed", "txId": "fa5670f38f45..."}
```

Check the namespace table was created in postgres:

```bash
docker exec -it netA-committer-p1-postgres \
  psql -U fabricx -d netA_p1 -c "\dt ns_*"
```

You should see `ns_token` and `ns__meta`.

## Running a second network (net B) on the same host

Repeat steps 2–6 with the **netB** port band from the table above and
`name: "netB"`. All resources are independent; only ports need to be
unique.

### Reusing organizations

If Party1MSP's org/CAs already exist from net A, just reuse `$ORG1`..`$ORG4`
in net B's orderer group, committer, and network JSON. You don't need
new orgs.

### Container name collisions

Container names are derived from the node's `name` field
(`netA-orderer-p1-router`, etc.). As long as your net A and net B node
names differ (`netA-*` vs `netB-*`), Docker will happily run both sets
side-by-side.

### Bind-mount directory collisions

Bind mounts are keyed by node name as well, under
`chaindeploy/data/fabricx-orderers/<node-name>/` and
`chaindeploy/data/fabricx-committers/<node-name>/`. Distinct node names
→ distinct directories.

## Tearing down a network

**⚠️ The built-in delete doesn't purge Docker state.** Do this manually:

```bash
# 1. Delete via API (drops DB rows)
curl -s -u "$AUTH" -X DELETE "$CL/api/v1/networks/fabricx/$NETA_ID"
for nid in $NODE_IDS; do
  curl -s -u "$AUTH" -X DELETE "$CL/api/v1/nodes/$nid"
done

# 2. Remove containers
docker ps -a --filter name=netA- -q | xargs -r docker rm -f

# 3. Remove bind mounts
rm -rf chaindeploy/data/fabricx-orderers/netA-*
rm -rf chaindeploy/data/fabricx-committers/netA-*

# 4. Remove volumes (if any)
docker volume prune -f
```

Skipping step 2 or 3 will cause `ABORTED_SIGNATURE_INVALID` on the next
rebuild because committers resume from a stale ledger position against
a freshly-regenerated genesis. See WALKTHROUGH.md §2 for the full story.

## Troubleshooting

| Symptom | Likely cause | Fix |
|--------|--------------|-----|
| `dial ... context deadline exceeded` on namespace create | `CHAINLAUNCH_FABRICX_LOCAL_DEV` not set on server | Kill server, re-export, restart |
| `invalid mount config ... bind source path does not exist` | Docker Desktop cold cache | Retry `--max-time 240`; first component warms the cache |
| `ABORTED_SIGNATURE_INVALID` | Stale ledger from prior run, or stale CA cert in `deployment_config` | Manual teardown (above); see WALKTHROUGH.md §2, §5 |
| TLS handshake failure despite server up | `deployment_config.tlsCaCert` drifted from org's current CA | SQL update to resync (see WALKTHROUGH.md §5) |
| Port already in use | Another network or service on same host | Pick a different port band |

## One-shot provisioning script

See `tmp/provision-fabricx.sh` in this repo for an end-to-end reference
script that does steps 1–6 for a single 4-party network. To adapt it
for multi-network, parameterize:

- `NETWORK_NAME` (e.g. `netA`, `netB`)
- `PORT_BASE` (e.g. `17000`, `17100`)
- `ORG_IDS` (reuse or create)

and run once per network.

## Glossary

| Term | Meaning |
|------|---------|
| **Party** | A participating organization. `partyId` is 1-indexed, max 10. |
| **Orderer group** | Router + batcher + consenter + assembler (one per party). |
| **Committer** | Sidecar + coordinator + validator + verifier + query-service + postgres (one per party). |
| **Assembler** | The orderer-group component that committers pull blocks from. |
| **Router** | The orderer-group entrypoint for client broadcasts (e.g., namespace tx). |
| **Channel** | Always `"arma"` for FabricX as of this writing. |
| **Namespace** | A logical partition within a channel; maps to a postgres table `ns_<name>`. |
