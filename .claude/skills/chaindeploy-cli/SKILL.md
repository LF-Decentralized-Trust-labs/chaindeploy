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

#### fabric orderer
Manage Fabric orderer nodes.
```bash
chainlaunch fabric orderer [create|list|update|delete]
```

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
**Flags:** `--name` (required), `--nodes`, `--org` (required), `--peerOrgs`, `--ordererOrgs`, `--channels`, `--peerCounts` (e.g. `Org1=2`), `--ordererCounts`, `--mode` (`service`|`docker`), `--external-ip`

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
