---
name: chaindeploy-cli
description: >
  CLI reference for ChainLaunch Core (chaindeploy). Covers all commands, subcommands, and flags
  for the `chainlaunch` binary. Use when building CLI features, debugging command issues,
  or helping users run chainlaunch commands.
---

# ChainLaunch Core CLI Reference

Binary: `chainlaunch` (built from `chaindeploy/`)

## Global Structure

```
chainlaunch [command]
```

No global flags in core edition.

## Commands

### serve
Start the API server.

```bash
chainlaunch serve [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `-p, --port` | `8100` | HTTP server port |
| `--db` | `~/.chainlaunch/chainlaunch.db` | SQLite database path |
| `--data` | `~/.chainlaunch` | Data directory path |
| `--tls-cert` | | TLS certificate file |
| `--tls-key` | | TLS key file |
| `--projects` | `projects-data` | Projects directory |
| `--dev` | `false` | Development mode |
| `--openai-key` | `$OPENAI_API_KEY` | OpenAI API key |
| `--anthropic-key` | `$ANTHROPIC_API_KEY` | Anthropic API key |
| `--ai-provider` | | AI provider (`openai` or `anthropic`) |
| `--ai-model` | | AI model name |

**Examples:**
```bash
chainlaunch serve --port 8100 --data ~/.chainlaunch --db chainlaunch.db --dev
chainlaunch serve --port 8100 --tls-cert cert.pem --tls-key key.pem
```

---

### fabric
Manage Hyperledger Fabric resources.

#### fabric peer
Manage Fabric peer nodes.
```bash
chainlaunch fabric peer [create|list|update|delete]
```
**create flags:** `--name`, `--msp-id`, `--org-id`, `--listen-addr`, `--chaincode-addr`, `--events-addr`, `--operations-addr`, `--external-addr`, `--version`, `--domain`, `--env`, `--address-override`, `--orderer-address-override`, `--mode` (`service`|`docker`, default: `service`)

#### fabric orderer
Manage Fabric orderer nodes.
```bash
chainlaunch fabric orderer [create|list|update|delete]
```
**create flags:** `--name`, `--msp-id`, `--org-id`, `--listen-addr`, `--admin-addr`, `--operations-addr`, `--external-addr`, `--version`, `--domain`, `--env`, `--mode` (`service`|`docker`, default: `service`)

#### fabric org
Manage Fabric organizations.
```bash
chainlaunch fabric org [create|list|update|delete]
```

#### fabric network-config pull
Pull network configuration from server.
```bash
chainlaunch fabric network-config pull [flags]
```
**Flags:** `--network` (name), `--msp-id`, `--output` (file), `--url` (API base URL), `--username`, `--password`

#### fabric install
Install/deploy chaincode to Fabric network.
```bash
chainlaunch fabric install [flags]
```
**Flags:** `--chaincode`, `--channel`, `--policy` (signature policy), `--chaincodeAddress`, `--envFile`, `--pdc` (private data collection JSON), `--metaInf`, `--rootCert`, `--clientCert`, `--clientKey`, `--local`

Network config files passed via positional args or flags (`--config`, `--user`, `--org`).

#### fabric invoke
Invoke a chaincode function.
```bash
chainlaunch fabric invoke [flags]
```
**Flags:** `--mspID`, `--user`, `--config` (network config), `--channel`, `--chaincode`, `--fcn`, `-a/--args`

#### fabric query
Query a chaincode function.
```bash
chainlaunch fabric query [flags]
```
**Flags:** `--mspID`, `--user`, `--config`, `--channel`, `--chaincode`, `--fcn`, `-a/--args`

---

### besu
Manage Besu nodes.

```bash
chainlaunch besu [create|list|update|delete]
```
Create, list, update, and delete Hyperledger Besu nodes.

---

### networks
Manage blockchain networks.

#### networks fabric
```bash
chainlaunch networks fabric [create|update|list|join|join-all|join-orderer|join-all-orderers]
```

- `join` — Join a peer to a network (`--network-id`, `--peer-id`)
- `join-all` — Join all peers to a network (`--network-id`)
- `join-orderer` — Join an orderer to a network (`--network-id`, `--orderer-id`)
- `join-all-orderers` — Join all orderers to a network (`--network-id`)

#### networks besu
```bash
chainlaunch networks besu [create|update|list]
```

---

### keys
Manage cryptographic keys.

```bash
chainlaunch keys [create|get]
```
Create and retrieve keys using various providers (Database, Vault, HSM).

---

### backup
Backup and restore operations.

#### backup restore
Restore a ChainLaunch instance from a restic backup.
```bash
chainlaunch backup restore [flags]
```
**Key Flags:** `--snapshot-id` (default: `latest`), `--repo-url`, `--aws-access-key`, `--aws-secret-key`, `--restic-password`, `--s3-endpoint`, `--bucket-name`, `--bucket-path`, `--s3-path-style`, `--output`, `--include-global`, `--exclude-config`, `--dry-run`, `--list-snapshots`, `--limit`, `--page`

---

### testnet
Create testnets for development/testing.

#### testnet fabric
```bash
chainlaunch testnet fabric [flags]
```
**Flags:** `--name` (required), `--nodes`, `--org` (required), `--peerOrgs`, `--ordererOrgs`, `--channels`, `--peerCounts` (e.g. `Org1=2`), `--ordererCounts`, `--mode` (`service`|`docker`), `--external-ip`, `--provider-id` (key provider ID, default: `1`)

#### testnet besu
```bash
chainlaunch testnet besu [flags]
```
**Flags:** `--name` (required), `--nodes` (min 4 for QBFT), `--prefix` (`besu`), `--mode` (`service`|`docker`), `--version` (`25.5.0`), `--initial-balance` (hex format)

---

### metrics
Manage Prometheus metrics integration.

```bash
chainlaunch metrics [enable|disable]
```

---

### version
Print version, git commit, and build time.

```bash
chainlaunch version
```

---

## Fabric Network Lifecycle Workflow

After creating a Fabric network (via `testnet fabric` or manually), you MUST complete ALL steps below. The network is NOT ready until anchor peers are set and chaincode is deployed. Always ask the user what type of smart contract they want to deploy.

**IMPORTANT**: When asked to "create a network", always go end-to-end through ALL these steps:

### 1. Create network
```bash
chainlaunch testnet fabric --name mynet --org Org1 --peerOrgs Org1 \
  --ordererOrgs Orderer1 --peerCounts Org1=2 --ordererCounts Orderer1=3 \
  --provider-id <KEY_PROVIDER_ID>
```
The `testnet` command automatically creates orgs, nodes, network, and joins all peers/orderers.

### 2. Verify nodes are running
```bash
# Check node status
curl -u $USER:$PASS $API_URL/nodes | jq '.items[] | {id, name, status, nodeType}'

# Check block height for a peer on a channel
curl -u $USER:$PASS $API_URL/nodes/<PEER_ID>/channels/<CHANNEL>/height
```
All orderers and peers should show `status: "RUNNING"`. Peers should have block height >= 1 after joining.

### 3. Set anchor peers (API only, no CLI command)
Anchor peers are required for cross-org gossip and service discovery. Without them, chaincodes cannot discover peers from other organizations.

```bash
curl -X POST -u $USER:$PASS $API_URL/networks/fabric/<NETWORK_ID>/anchor-peers \
  -H 'Content-Type: application/json' \
  -d '{
    "organizationId": <ORG_ID>,
    "anchorPeers": [{"host": "127.0.0.1", "port": 7051}]
  }'
```
Repeat for each peer organization. Use the peer's external endpoint host and port.

### 4. Pull connection profile
```bash
chainlaunch fabric network-config pull \
  --network <NETWORK_NAME> --msp-id <MSP_ID> \
  --output connection-profile.yaml \
  --url http://localhost:8100/api/v1 --username admin --password admin123
```

### 5. Install chaincode (full lifecycle: install + approve + commit)
The `fabric install` command handles the complete Fabric chaincode lifecycle in one step.
The chaincode runs as a service (ccaas) — you provide the address where it will listen.

```bash
chainlaunch fabric install \
  --chaincode tokenizer \
  --channel mychannel \
  --policy "OR('Org1MSP.member')" \
  --chaincodeAddress "127.0.0.1:9999" \
  --config connection-profile.yaml \
  -u admin \
  -o Org1MSP \
  --local
```

**Flags:** `--chaincode` (name), `--channel`, `--policy` (endorsement policy), `--chaincodeAddress` (where chaincode server listens), `--config` (connection profile, one per org), `-u/--users`, `-o/--organizations`, `--local` (skip tunnel, use direct address), `--envFile` (write env vars), `--pdc` (private data collection JSON), `--metaInf`, `--rootCert`, `--clientCert`, `--clientKey`

After install, start your chaincode binary with:
- `CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999` — address to listen on
- `CORE_CHAINCODE_ID_NAME=<name>:<packageID>` — from install output
- `CORE_PEER_TLS_ENABLED=false` — for local development

### 6. Invoke and query chaincode
```bash
# Invoke (writes to ledger)
chainlaunch fabric invoke --mspID Org1MSP --user admin --config connection-profile.yaml \
  --channel mychannel --chaincode tokenizer --fcn Transfer -a token1 -a token2 -a 200

# Query (reads from ledger)
chainlaunch fabric query --mspID Org1MSP --user admin --config connection-profile.yaml \
  --channel mychannel --chaincode tokenizer --fcn GetAsset -a token1
```

### API-Only Operations (no CLI equivalent)

These operations are available via REST API but have no CLI commands:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/networks/fabric/{id}/anchor-peers` | Set anchor peers for an org |
| `GET` | `/networks/fabric/{id}/blocks` | List blocks |
| `GET` | `/networks/fabric/{id}/blocks/{blockNum}` | Get block transactions |
| `GET` | `/networks/fabric/{id}/channel-config` | Get channel configuration |
| `GET` | `/networks/fabric/{id}/info` | Get chain info (height, etc.) |
| `GET` | `/networks/fabric/{id}/map` | Get network topology map |
| `GET` | `/nodes/{id}/channels` | List channels a node has joined |
| `GET` | `/nodes/{id}/channels/{channel}/height` | Get block height for a channel |
| `GET` | `/nodes/{id}/channels/{channel}/chaincodes` | List installed chaincodes |
| `POST` | `/networks/fabric/{id}/update-config` | Prepare config update |
| `POST` | `/networks/fabric/{id}/reload-block` | Reload config block |

---

## Environment Variables

| Variable | Used By | Description |
|----------|---------|-------------|
| `CHAINLAUNCH_API_URL` | CLI commands | API base URL (default: `http://localhost:8100/api/v1`) |
| `CHAINLAUNCH_USER` | CLI commands | Basic auth username |
| `CHAINLAUNCH_PASSWORD` | CLI commands | Basic auth password |
| `OPENAI_API_KEY` | `serve` | OpenAI API key |
| `ANTHROPIC_API_KEY` | `serve` | Anthropic API key |

## Source Location

All CLI commands are in `chaindeploy/cmd/` with subpackages per command group.
