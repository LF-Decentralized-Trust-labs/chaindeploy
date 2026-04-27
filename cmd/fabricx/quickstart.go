// Package fabricx — see fabricx.go for the package purpose. This file
// implements `chainlaunch fabricx quickstart`, the CLI twin of the
// /networks/fabricx/quickstart page in the web UI.
//
// The flow is the exact same 8-phase sequence the browser runs (see
// web/src/pages/networks/fabricx/quickstart.tsx):
//
//  1. Ensure PartyNMSP organizations (1..N)
//  2. Create a shared Postgres service (one container, N databases)
//  3. Start the Postgres container
//  4. Provision per-party databases + roles
//  5. Create + init orderer node group per party (generates certs + 4 children)
//  6. Create committer node per party (stage 1 — certs only)
//  7. Create the FabricX network (generates Arma genesis)
//  8. Join all 20 nodes (4 orderer children + 1 committer, × N parties)
//
// Hardening over the naive translation of the UI flow:
//
//   - Every HTTP call has a per-request timeout (--http-timeout). The backend
//     has been observed hanging on the FabricX join path during the macOS
//     Docker bind-mount race; without a timeout the CLI would wait forever.
//   - Joins retry with exponential backoff (--join-retries / --join-retry-backoff).
//     Transient Docker errors during Container.Create are the dominant failure
//     mode; one retry is usually enough to clear them.
//   - After each join, the CLI fetches the node's status from the API and
//     flags anything that is not "joined" as a warning, because the backend
//     currently writes genesis and flips status even when the container start
//     fails. This catches the "server says joined but nothing is running"
//     class of bugs the UI can't detect.
//   - --clean tears down any prior bundle with this network name before
//     provisioning (via API DELETEs, no sqlite poking), so reruns don't hit
//     UNIQUE constraint failures on node_groups.name.
package fabricx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/spf13/cobra"
)

// Default port layout — matches constants in the web UI quickstart page.
// Changing these here without updating the UI creates divergent bundles.
const (
	defaultBasePort           = 17000
	defaultSlotSize           = 20
	defaultPostgresPort       = 15432
	defaultNumParties         = 4
	defaultChannelID          = "arma"
	defaultNetworkName        = "fabricx-quickstart"
	sharedPostgresServiceName = "fabricx-quickstart-pg"
	sharedPostgresNetworkName = "fabricx-quickstart-pg-net"
	sharedPostgresAdminUser   = "postgres"
	sharedPostgresAdminPass   = "postgres"

	defaultHTTPTimeout      = 3 * time.Minute
	defaultJoinRetries      = 2
	defaultJoinRetryBackoff = 5 * time.Second
)

type partyPorts struct {
	router, batcher, consenter, assembler                   int
	sidecar, coordinator, validator, verifier, queryService int
}

func portsForParty(basePort, slotSize, i int) partyPorts {
	base := basePort + i*slotSize
	return partyPorts{
		router: base, batcher: base + 1, consenter: base + 2, assembler: base + 3,
		sidecar: base + 10, coordinator: base + 11, validator: base + 12,
		verifier: base + 13, queryService: base + 14,
	}
}

type partyDB struct {
	DB       string `json:"db"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func partyDatabaseSpec(partyID int) partyDB {
	return partyDB{
		DB:       fmt.Sprintf("party%d_fabricx", partyID),
		User:     fmt.Sprintf("party%d", partyID),
		Password: fmt.Sprintf("party%dpw", partyID),
	}
}

type partyResult struct {
	partyID               int
	mspID                 string
	organizationID        int64
	ordererNodeGroupID    int64
	ordererChildNodeIDs   []int64 // router, batcher, consenter, assembler
	committerNodeGroupID  int64
	committerChildNodeIDs []int64 // sidecar, coordinator, validator, verifier, query-service
}

// apiClient is a local HTTP wrapper with per-request timeout. The shared
// cmd/common.Client uses http.Client{} with no timeout, which we can't change
// without rippling into every existing CLI subcommand. This keeps the fabricx
// quickstart hardened without touching that shared code.
type apiClient struct {
	http     *http.Client
	baseURL  string
	username string
	password string
}

func newAPIClientFromEnv(timeout time.Duration) (*apiClient, error) {
	apiURL := os.Getenv("CHAINLAUNCH_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8100/api/v1"
	}
	user := os.Getenv("CHAINLAUNCH_USER")
	if user == "" {
		return nil, fmt.Errorf("CHAINLAUNCH_USER is not set")
	}
	pw := os.Getenv("CHAINLAUNCH_PASSWORD")
	if pw == "" {
		return nil, fmt.Errorf("CHAINLAUNCH_PASSWORD is not set")
	}
	return &apiClient{
		http:     &http.Client{Timeout: timeout},
		baseURL:  apiURL,
		username: user,
		password: pw,
	}, nil
}

func (c *apiClient) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("build %s %s: %w", method, path, err)
	}
	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// Wrap timeouts so callers can classify them.
		var uerr *urlErrorLike
		if errors.As(err, &uerr) || os.IsTimeout(err) {
			return nil, fmt.Errorf("%s %s timed out after %s: %w", method, path, c.http.Timeout, err)
		}
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	return resp, nil
}

// urlErrorLike is a placeholder so errors.As can be called; kept nil by
// intent (net/url.Error is matched via the standard library). Defining it
// avoids an import cycle in toolchains that disagree on the public field.
type urlErrorLike struct{ _ struct{} }

func (*urlErrorLike) Error() string { return "" }

func (c *apiClient) get(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}
func (c *apiClient) post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, path, body)
}
func (c *apiClient) delete_(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodDelete, path, nil)
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func readErrorBody(resp *http.Response) string {
	b, _ := readBody(resp)
	msg := strings.TrimSpace(string(b))
	if msg == "" {
		return fmt.Sprintf("HTTP %d (empty body)", resp.StatusCode)
	}
	return fmt.Sprintf("HTTP %d: %s", resp.StatusCode, msg)
}

func newQuickstartCmd(log *logger.Logger) *cobra.Command {
	_ = log // reserved for future structured logging; CLI currently prints progress inline
	var (
		networkName      string
		externalIP       string
		basePort         int
		slotSize         int
		postgresPort     int
		numParties       int
		channelID        string
		keepGoing        bool
		clean            bool
		httpTimeout      time.Duration
		joinRetries      int
		joinRetryBackoff time.Duration
		namespaceName    string
		namespaceTimeout time.Duration
		dataPath         string
	)

	cmd := &cobra.Command{
		Use:   "quickstart",
		Short: "Provision a 4-party FabricX network (Arma consensus) in one shot",
		Long: `Provision a complete FabricX 4-party network — the same bundle the "FabricX
Quick Start" button in the web UI creates.

Requires CHAINLAUNCH_USER and CHAINLAUNCH_PASSWORD for basic auth; and a running
chainlaunch server (default http://localhost:8100, override with CHAINLAUNCH_API_URL).

External IP: if --external-ip is omitted the CLI reads the platform default
from GET /nodes/defaults/besu-node (same source the Fabric/Besu node flows use).

Hardening:
  --http-timeout       per-HTTP-request deadline (prevents indefinite hangs
                       on the FabricX join path during the macOS Docker race).
  --join-retries       retry count for transient Docker errors on node join.
  --join-retry-backoff delay between join retries.
  --clean              tear down any prior bundle with this network name
                       before provisioning (API deletes — no sqlite poking).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			c, err := newAPIClientFromEnv(httpTimeout)
			if err != nil {
				return err
			}

			if externalIP == "" {
				externalIP, err = fetchDefaultExternalIP(ctx, c)
				if err != nil {
					return fmt.Errorf("failed to discover external IP (pass --external-ip to override): %w", err)
				}
				if externalIP == "" {
					return fmt.Errorf("no external IP configured. Set one under Settings → Network in the UI, or pass --external-ip")
				}
				fmt.Fprintf(os.Stdout, "Using external IP %s (from platform defaults)\n", externalIP)
			}

			cfg := quickstartConfig{
				networkName:      networkName,
				externalIP:       externalIP,
				basePort:         basePort,
				slotSize:         slotSize,
				postgresPort:     postgresPort,
				numParties:       numParties,
				channelID:        channelID,
				keepGoing:        keepGoing,
				joinRetries:      joinRetries,
				joinRetryBackoff: joinRetryBackoff,
				namespaceName:    namespaceName,
				namespaceTimeout: namespaceTimeout,
				dataPath:         dataPath,
			}

			if clean {
				if err := cleanBundle(ctx, c, cfg); err != nil {
					return fmt.Errorf("--clean failed: %w", err)
				}
			}

			return runQuickstart(ctx, c, cfg)
		},
	}

	cmd.Flags().StringVar(&networkName, "network-name", defaultNetworkName, "Name of the FabricX network to create")
	cmd.Flags().StringVar(&externalIP, "external-ip", "", "External IP/host for node endpoints (default: platform setting)")
	cmd.Flags().IntVar(&basePort, "base-port", defaultBasePort, "First port in the reserved range")
	cmd.Flags().IntVar(&slotSize, "slot-size", defaultSlotSize, "Ports reserved per party")
	cmd.Flags().IntVar(&postgresPort, "postgres-port", defaultPostgresPort, "Host port for the shared Postgres container")
	cmd.Flags().IntVar(&numParties, "parties", defaultNumParties, "Number of parties (defaults to 4)")
	cmd.Flags().StringVar(&channelID, "channel", defaultChannelID, "Channel ID (FabricX locks this to 'arma')")
	cmd.Flags().BoolVar(&keepGoing, "keep-going", true, "Continue joining nodes even if one hits a transient Docker error")
	cmd.Flags().BoolVar(&clean, "clean", false, "Wipe any prior bundle with this network name before provisioning")
	cmd.Flags().DurationVar(&httpTimeout, "http-timeout", defaultHTTPTimeout, "Per-HTTP-request deadline")
	cmd.Flags().IntVar(&joinRetries, "join-retries", defaultJoinRetries, "Retry count for transient Docker errors during node join")
	cmd.Flags().DurationVar(&joinRetryBackoff, "join-retry-backoff", defaultJoinRetryBackoff, "Delay between join retries (doubles each retry)")
	cmd.Flags().StringVar(&namespaceName, "namespace", "quickstart", "Namespace created as a post-provisioning health check (empty to skip)")
	cmd.Flags().DurationVar(&namespaceTimeout, "namespace-timeout", 60*time.Second, "Committer finality timeout for the health-check namespace")
	cmd.Flags().StringVar(&dataPath, "data-path", "", "Server --data directory; when combined with --clean, purges fabricx-orderers/ and fabricx-committers/ bind-mounts so stale TLS certs don't survive a rerun")

	return cmd
}

type quickstartConfig struct {
	networkName      string
	externalIP       string
	basePort         int
	slotSize         int
	postgresPort     int
	numParties       int
	channelID        string
	keepGoing        bool
	joinRetries      int
	joinRetryBackoff time.Duration
	// namespaceName is the namespace created as a health-check after all
	// joins. Empty string skips the health check; default is "quickstart". A
	// committed namespace proves the full path works: router → consenter →
	// batcher → assembler → sidecar → coordinator → validator → DB.
	namespaceName    string
	namespaceTimeout time.Duration
	// dataPath points at the server's --data directory. When set and --clean
	// is used, the CLI purges ${dataPath}/fabricx-orderers and
	// ${dataPath}/fabricx-committers before re-provisioning. Required to
	// avoid stale on-disk TLS certs surviving a rerun and colliding with
	// the freshly-generated DB keys that go into the new genesis block.
	dataPath string
}

// runQuickstart is the orchestrator. Each phase prints a one-line status so the
// operator can follow progress in CI logs; failures include the HTTP body so
// the root cause is visible without re-reading the server log.
func runQuickstart(ctx context.Context, c *apiClient, cfg quickstartConfig) error {
	if cfg.channelID != defaultChannelID {
		return fmt.Errorf("FabricX requires --channel=%s (got %q)", defaultChannelID, cfg.channelID)
	}

	parties := make([]partyResult, cfg.numParties)
	for i := range parties {
		parties[i] = partyResult{partyID: i + 1, mspID: fmt.Sprintf("Party%dMSP", i+1)}
	}

	// Phase 1: orgs
	for i := range parties {
		status("Ensuring organization %s", parties[i].mspID)
		orgID, err := findOrCreateOrg(ctx, c, parties[i].mspID)
		if err != nil {
			return fmt.Errorf("org %s: %w", parties[i].mspID, err)
		}
		parties[i].organizationID = orgID
		done("  → org #%d", orgID)
	}

	// Phase 2-4: shared postgres
	status("Creating shared Postgres service %s", sharedPostgresServiceName)
	pgID, err := findOrCreatePostgresService(ctx, c, cfg.postgresPort)
	if err != nil {
		return fmt.Errorf("postgres service: %w", err)
	}
	done("  → service #%d", pgID)

	status("Starting shared Postgres container (host :%d)", cfg.postgresPort)
	if err := startPostgresService(ctx, c, pgID); err != nil {
		return fmt.Errorf("start postgres: %w", err)
	}
	// Mirror UI behavior: small pause so CREATE ROLE doesn't race startup.
	time.Sleep(2 * time.Second)
	done("  → running")

	status("Provisioning %d per-party databases/roles", cfg.numParties)
	if err := provisionPartyDatabases(ctx, c, pgID, cfg.numParties); err != nil {
		return fmt.Errorf("provision databases: %w", err)
	}
	done("  → %d databases", cfg.numParties)

	// Phase 5: orderer node groups (ADR-0001)
	for i := range parties {
		status("Creating + initializing orderer group for %s", parties[i].mspID)
		groupID, childIDs, err := createOrdererNodeGroup(ctx, c, parties[i], cfg)
		if err != nil {
			return fmt.Errorf("orderer group %s: %w", parties[i].mspID, err)
		}
		parties[i].ordererNodeGroupID = groupID
		parties[i].ordererChildNodeIDs = childIDs
		done("  → group #%d, children %v", groupID, childIDs)
	}

	// Phase 6: committers
	for i := range parties {
		status("Creating committer node-group for %s", parties[i].mspID)
		groupID, childIDs, err := createCommitterNodeGroup(ctx, c, parties[i], cfg)
		if err != nil {
			return fmt.Errorf("committer %s: %w", parties[i].mspID, err)
		}
		parties[i].committerNodeGroupID = groupID
		parties[i].committerChildNodeIDs = childIDs
		done("  → group #%d, children %v", groupID, childIDs)
	}

	// Phase 7: network (genesis)
	status("Creating FabricX network + Arma genesis block")
	networkID, err := createNetwork(ctx, c, cfg, parties)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}
	done("  → network #%d", networkID)

	// Phase 8: joins with retry + post-join status verification.
	var joinErrors []string
	var unhealthy []string
	for i := range parties {
		status("Joining %s orderer children (router→batcher→consenter→assembler)", parties[i].mspID)
		for _, childID := range parties[i].ordererChildNodeIDs {
			if err := joinNodeWithRetry(ctx, c, networkID, childID, cfg); err != nil {
				joinErrors = append(joinErrors, fmt.Sprintf("orderer %s child #%d: %v", parties[i].mspID, childID, err))
				if !cfg.keepGoing {
					return fmt.Errorf("join orderer child #%d: %w", childID, err)
				}
				fail("  ! child #%d: %v (continuing)", childID, err)
				continue
			}
			if healthy, detail := verifyNodeJoined(ctx, c, childID); !healthy {
				unhealthy = append(unhealthy, fmt.Sprintf("orderer %s child #%d: %s", parties[i].mspID, childID, detail))
				warn("  ⚠ child #%d joined but %s", childID, detail)
			} else {
				done("  → child #%d joined", childID)
			}
		}
	}
	for i := range parties {
		status("Joining %s committer children (verifier→validator→coordinator→sidecar→query-service)", parties[i].mspID)
		for _, childID := range parties[i].committerChildNodeIDs {
			if err := joinNodeWithRetry(ctx, c, networkID, childID, cfg); err != nil {
				joinErrors = append(joinErrors, fmt.Sprintf("committer %s child #%d: %v", parties[i].mspID, childID, err))
				if !cfg.keepGoing {
					return fmt.Errorf("join committer child #%d: %w", childID, err)
				}
				fail("  ! child #%d: %v (continuing)", childID, err)
				continue
			}
			if healthy, detail := verifyNodeJoined(ctx, c, childID); !healthy {
				unhealthy = append(unhealthy, fmt.Sprintf("committer %s child #%d: %s", parties[i].mspID, childID, detail))
				warn("  ⚠ child #%d joined but %s", childID, detail)
			} else {
				done("  → child #%d joined", childID)
			}
		}
	}

	// Phase 9: reconcile. The backend's JoinNode swallows transient Docker
	// bind-mount errors (deployer.go isTransientDockerMountErr) and leaves
	// the DB saying "joined" while 3–11 of the ~20 containers never actually
	// started. POST /nodes/{id}/start is idempotent, so we issue it against
	// every node after all joins have completed; by then the Docker apiproxy
	// queue has drained and the retry actually creates the containers.
	status("Reconciling node containers (post-join /start retry)")
	allNodeIDs := make([]int64, 0, cfg.numParties*9)
	for i := range parties {
		allNodeIDs = append(allNodeIDs, parties[i].ordererChildNodeIDs...)
		allNodeIDs = append(allNodeIDs, parties[i].committerChildNodeIDs...)
	}
	var reconcileErrors []string
	for _, nid := range allNodeIDs {
		if err := startNodeWithRetry(ctx, c, nid, cfg); err != nil {
			reconcileErrors = append(reconcileErrors, fmt.Sprintf("node #%d: %v", nid, err))
			warn("  ⚠ reconcile node #%d: %v", nid, err)
			continue
		}
	}
	if len(reconcileErrors) == 0 {
		done("  → all %d nodes reconciled", len(allNodeIDs))
	}

	// Phase 10: namespace health check. A namespace creation is the shortest
	// path that exercises the entire data plane (router signing, consenter
	// ordering, batcher batching, assembler assembly, committer finality). If
	// the network is not fully wired — stale TLS cert, exited container,
	// ledger divergence — this step fails. Skipped on --namespace="".
	if cfg.namespaceName != "" {
		status("Health check: creating namespace %q (waits for finality)", cfg.namespaceName)
		submitterOrgID := parties[0].organizationID
		if err := createNamespace(ctx, c, networkID, cfg, submitterOrgID); err != nil {
			fmt.Printf("\n⚠ namespace health check failed: %v\n", err)
			fmt.Println("   The data plane is not committed end-to-end. Check `docker ps -a | grep fabricx` for exited")
			fmt.Println("   containers, and `docker logs <container>` for the root cause.")
			return fmt.Errorf("namespace health check failed: %w", err)
		}
		done("  → namespace %q committed", cfg.namespaceName)
	}

	fmt.Printf("\n✓ FabricX network #%d (%q) provisioned with %d parties\n",
		networkID, cfg.networkName, cfg.numParties)
	for _, p := range parties {
		pp := portsForParty(cfg.basePort, cfg.slotSize, p.partyID-1)
		fmt.Printf("  %s: orderer-group=#%d (ports %d-%d), committer-group=#%d (ports %d-%d)\n",
			p.mspID, p.ordererNodeGroupID, pp.router, pp.assembler,
			p.committerNodeGroupID, pp.sidecar, pp.queryService)
	}
	fmt.Printf("  Shared Postgres: host :%d (%d databases)\n", cfg.postgresPort, cfg.numParties)

	if len(joinErrors) > 0 || len(unhealthy) > 0 {
		if len(joinErrors) > 0 {
			fmt.Printf("\n⚠ %d join(s) returned errors:\n", len(joinErrors))
			for _, e := range joinErrors {
				fmt.Printf("  - %s\n", e)
			}
		}
		if len(unhealthy) > 0 {
			fmt.Printf("\n⚠ %d node(s) joined but are not healthy (server flipped status but container is not running):\n", len(unhealthy))
			for _, e := range unhealthy {
				fmt.Printf("  - %s\n", e)
			}
		}
		return fmt.Errorf("%d join error(s), %d unhealthy node(s)", len(joinErrors), len(unhealthy))
	}
	return nil
}

// ---------- helpers ----------

func status(format string, a ...any) { fmt.Printf("→ "+format+"\n", a...) }
func done(format string, a ...any)   { fmt.Printf(format+"\n", a...) }
func fail(format string, a ...any)   { fmt.Printf(format+"\n", a...) }
func warn(format string, a ...any)   { fmt.Printf(format+"\n", a...) }

// orgListResponse matches the JSON envelope returned by GET /organizations.
type orgListResponse struct {
	Items []struct {
		ID    int64  `json:"id"`
		MspID string `json:"mspId"`
	} `json:"items"`
}

func fetchDefaultExternalIP(ctx context.Context, c *apiClient) (string, error) {
	resp, err := c.get(ctx, "/nodes/defaults/besu-node")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New(readErrorBody(resp))
	}
	var body struct {
		Defaults []struct {
			ExternalIP string `json:"externalIp"`
		} `json:"defaults"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode defaults: %w", err)
	}
	if len(body.Defaults) == 0 {
		return "", nil
	}
	return body.Defaults[0].ExternalIP, nil
}

func findOrCreateOrg(ctx context.Context, c *apiClient, mspID string) (int64, error) {
	resp, err := c.get(ctx, "/organizations?limit=1000")
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := readBody(resp)
		return 0, fmt.Errorf("list orgs: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var list orgListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		resp.Body.Close()
		return 0, fmt.Errorf("decode org list: %w", err)
	}
	resp.Body.Close()
	for _, o := range list.Items {
		if o.MspID == mspID {
			return o.ID, nil
		}
	}

	body := map[string]any{
		"mspId":       mspID,
		"name":        mspID,
		"description": "Auto-created by fabricx quickstart",
	}
	cresp, err := c.post(ctx, "/organizations", body)
	if err != nil {
		return 0, err
	}
	defer cresp.Body.Close()
	if cresp.StatusCode != http.StatusCreated {
		return 0, errors.New(readErrorBody(cresp))
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(cresp.Body).Decode(&created); err != nil {
		return 0, fmt.Errorf("decode created org: %w", err)
	}
	if created.ID == 0 {
		return 0, fmt.Errorf("server returned no id for %s", mspID)
	}
	return created.ID, nil
}

func findOrCreatePostgresService(ctx context.Context, c *apiClient, hostPort int) (int64, error) {
	body := map[string]any{
		"name":     sharedPostgresServiceName,
		"db":       "postgres",
		"user":     sharedPostgresAdminUser,
		"password": sharedPostgresAdminPass,
		"hostPort": hostPort,
	}
	resp, err := c.post(ctx, "/services/postgres", body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		var created struct {
			ID int64 `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
			return 0, fmt.Errorf("decode created service: %w", err)
		}
		return created.ID, nil
	}
	b, _ := readBody(resp)
	if strings.Contains(string(b), "UNIQUE") || strings.Contains(string(b), "already") || resp.StatusCode == http.StatusConflict {
		return findPostgresServiceByName(ctx, c, sharedPostgresServiceName)
	}
	return 0, fmt.Errorf("create postgres service: %s", strings.TrimSpace(string(b)))
}

func findPostgresServiceByName(ctx context.Context, c *apiClient, name string) (int64, error) {
	resp, err := c.get(ctx, "/services?serviceType=POSTGRES&limit=500")
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := readBody(resp)
		return 0, fmt.Errorf("list services: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	body, err := readBody(resp)
	if err != nil {
		return 0, err
	}
	var list []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		var env struct {
			Items []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		}
		if err2 := json.Unmarshal(body, &env); err2 != nil {
			return 0, fmt.Errorf("decode services list: %v / %v", err, err2)
		}
		list = env.Items
	}
	for _, s := range list {
		if s.Name == name {
			return s.ID, nil
		}
	}
	return 0, fmt.Errorf("service %q not found", name)
}

func startPostgresService(ctx context.Context, c *apiClient, serviceID int64) error {
	body := map[string]any{"networkName": sharedPostgresNetworkName}
	resp, err := c.post(ctx, fmt.Sprintf("/services/%d/start", serviceID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	b, _ := readBody(resp)
	msg := strings.ToLower(string(b))
	if strings.Contains(msg, "already") || strings.Contains(msg, "running") {
		return nil
	}
	return fmt.Errorf("start service %d: %s", serviceID, strings.TrimSpace(string(b)))
}

func provisionPartyDatabases(ctx context.Context, c *apiClient, serviceID int64, numParties int) error {
	dbs := make([]partyDB, 0, numParties)
	for i := 1; i <= numParties; i++ {
		dbs = append(dbs, partyDatabaseSpec(i))
	}
	body := map[string]any{"databases": dbs}
	resp, err := c.post(ctx, fmt.Sprintf("/services/%d/postgres/databases", serviceID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusCreated {
		return nil
	}
	return errors.New(readErrorBody(resp))
}

func createOrdererNodeGroup(ctx context.Context, c *apiClient, p partyResult, cfg quickstartConfig) (int64, []int64, error) {
	pp := portsForParty(cfg.basePort, cfg.slotSize, p.partyID-1)
	groupName := strings.ToLower(p.mspID) + "-orderer"

	createBody := map[string]any{
		"name":           groupName,
		"platform":       "FABRICX",
		"groupType":      "FABRICX_ORDERER_GROUP",
		"organizationId": p.organizationID,
		"mspId":          p.mspID,
		"partyId":        p.partyID,
		"externalIp":     cfg.externalIP,
		"domainNames":    []string{cfg.externalIP, "localhost"},
	}
	resp, err := c.post(ctx, "/node-groups", createBody)
	if err != nil {
		return 0, nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		defer resp.Body.Close()
		return 0, nil, fmt.Errorf("create node_group %s: %s", groupName, readErrorBody(resp))
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		return 0, nil, fmt.Errorf("decode node_group: %w", err)
	}
	resp.Body.Close()
	if created.ID == 0 {
		return 0, nil, fmt.Errorf("server returned no id for node_group %s", groupName)
	}
	groupID := created.ID

	initBody := map[string]any{
		"routerPort":    pp.router,
		"batcherPort":   pp.batcher,
		"consenterPort": pp.consenter,
		"assemblerPort": pp.assembler,
	}
	iresp, err := c.post(ctx, fmt.Sprintf("/node-groups/%d/init", groupID), initBody)
	if err != nil {
		return 0, nil, err
	}
	iresp.Body.Close()
	if iresp.StatusCode != http.StatusOK && iresp.StatusCode != http.StatusCreated {
		return 0, nil, fmt.Errorf("init node_group %d: HTTP %d", groupID, iresp.StatusCode)
	}

	cresp, err := c.get(ctx, fmt.Sprintf("/node-groups/%d/children", groupID))
	if err != nil {
		return 0, nil, err
	}
	if cresp.StatusCode != http.StatusOK {
		defer cresp.Body.Close()
		return 0, nil, fmt.Errorf("fetch children %d: %s", groupID, readErrorBody(cresp))
	}
	var children []struct {
		ID       int64  `json:"id"`
		NodeType string `json:"nodeType"`
	}
	if err := json.NewDecoder(cresp.Body).Decode(&children); err != nil {
		cresp.Body.Close()
		return 0, nil, fmt.Errorf("decode children: %w", err)
	}
	cresp.Body.Close()
	order := []string{
		"FABRICX_ORDERER_ROUTER",
		"FABRICX_ORDERER_BATCHER",
		"FABRICX_ORDERER_CONSENTER",
		"FABRICX_ORDERER_ASSEMBLER",
	}
	childIDs := make([]int64, 0, 4)
	for _, role := range order {
		var found int64
		for _, ch := range children {
			if ch.NodeType == role {
				found = ch.ID
				break
			}
		}
		if found == 0 {
			return 0, nil, fmt.Errorf("group %d missing child for role %s", groupID, role)
		}
		childIDs = append(childIDs, found)
	}
	return groupID, childIDs, nil
}

// createCommitterNodeGroup creates a FABRICX_COMMITTER node_group plus
// its 5 per-role child rows (sidecar, coordinator, validator, verifier,
// query-service) via the same /node-groups/{id}/init dispatch the
// orderer side uses. Returns the parent group ID and the ordered list
// of child node IDs.
func createCommitterNodeGroup(ctx context.Context, c *apiClient, p partyResult, cfg quickstartConfig) (int64, []int64, error) {
	pp := portsForParty(cfg.basePort, cfg.slotSize, p.partyID-1)
	db := partyDatabaseSpec(p.partyID)
	groupName := strings.ToLower(p.mspID) + "-committer"

	createBody := map[string]any{
		"name":           groupName,
		"platform":       "FABRICX",
		"groupType":      "FABRICX_COMMITTER",
		"organizationId": p.organizationID,
		"mspId":          p.mspID,
		"externalIp":     cfg.externalIP,
		"domainNames":    []string{cfg.externalIP, "localhost"},
	}
	resp, err := c.post(ctx, "/node-groups", createBody)
	if err != nil {
		return 0, nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		defer resp.Body.Close()
		return 0, nil, fmt.Errorf("create node_group %s: %s", groupName, readErrorBody(resp))
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		return 0, nil, fmt.Errorf("decode node_group: %w", err)
	}
	resp.Body.Close()
	if created.ID == 0 {
		return 0, nil, fmt.Errorf("server returned no id for node_group %s", groupName)
	}
	groupID := created.ID

	initBody := map[string]any{
		"sidecarPort":      pp.sidecar,
		"coordinatorPort":  pp.coordinator,
		"validatorPort":    pp.validator,
		"verifierPort":     pp.verifier,
		"queryServicePort": pp.queryService,
		"postgresHost":     cfg.externalIP,
		"postgresPort":     cfg.postgresPort,
		"postgresDb":       db.DB,
		"postgresUser":     db.User,
		"postgresPassword": db.Password,
		"channelId":        cfg.channelID,
		"ordererEndpoints": []string{fmt.Sprintf("%s:%d", cfg.externalIP, pp.assembler)},
	}
	iresp, err := c.post(ctx, fmt.Sprintf("/node-groups/%d/init", groupID), initBody)
	if err != nil {
		return 0, nil, err
	}
	iresp.Body.Close()
	if iresp.StatusCode != http.StatusOK && iresp.StatusCode != http.StatusCreated {
		return 0, nil, fmt.Errorf("init node_group %d: HTTP %d", groupID, iresp.StatusCode)
	}

	cresp, err := c.get(ctx, fmt.Sprintf("/node-groups/%d/children", groupID))
	if err != nil {
		return 0, nil, err
	}
	if cresp.StatusCode != http.StatusOK {
		defer cresp.Body.Close()
		return 0, nil, fmt.Errorf("fetch children %d: %s", groupID, readErrorBody(cresp))
	}
	var children []struct {
		ID       int64  `json:"id"`
		NodeType string `json:"nodeType"`
	}
	if err := json.NewDecoder(cresp.Body).Decode(&children); err != nil {
		cresp.Body.Close()
		return 0, nil, fmt.Errorf("decode children: %w", err)
	}
	cresp.Body.Close()
	// Order matches StartGroup's ChildRoles ordering: verifier and
	// validator first (downstream of coordinator), then coordinator,
	// then sidecar (which dials the orderer), and finally query-service
	// (read-only). Mirrors ngtypes.ChildRoles(GroupTypeFabricXCommitter).
	order := []string{
		"FABRICX_COMMITTER_VERIFIER",
		"FABRICX_COMMITTER_VALIDATOR",
		"FABRICX_COMMITTER_COORDINATOR",
		"FABRICX_COMMITTER_SIDECAR",
		"FABRICX_COMMITTER_QUERY_SERVICE",
	}
	childIDs := make([]int64, 0, 5)
	for _, role := range order {
		var found int64
		for _, ch := range children {
			if ch.NodeType == role {
				found = ch.ID
				break
			}
		}
		if found == 0 {
			return 0, nil, fmt.Errorf("group %d missing child for role %s", groupID, role)
		}
		childIDs = append(childIDs, found)
	}
	return groupID, childIDs, nil
}

func createNetwork(ctx context.Context, c *apiClient, cfg quickstartConfig, parties []partyResult) (int64, error) {
	orgs := make([]map[string]any, 0, len(parties))
	for _, p := range parties {
		orgs = append(orgs, map[string]any{
			"id":                   p.organizationID,
			"ordererNodeGroupId":   p.ordererNodeGroupID,
			"committerNodeGroupId": p.committerNodeGroupID,
		})
	}
	body := map[string]any{
		"name":        cfg.networkName,
		"description": "FabricX 4-party quickstart network (from CLI)",
		"config": map[string]any{
			"channelName":   cfg.channelID,
			"organizations": orgs,
		},
	}
	resp, err := c.post(ctx, "/networks/fabricx", body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("create network: %s", readErrorBody(resp))
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return 0, fmt.Errorf("decode network: %w", err)
	}
	if created.ID == 0 {
		return 0, fmt.Errorf("server returned no id for network")
	}
	return created.ID, nil
}

// joinNodeWithRetry calls POST /networks/fabricx/{netID}/nodes/{nodeID}/join
// with retry + exponential backoff. The dominant failure mode is a Docker
// bind-mount race on macOS that surfaces as "bind source path does not exist"
// — usually clears on a second attempt once the FS has settled.
func joinNodeWithRetry(ctx context.Context, c *apiClient, networkID, nodeID int64, cfg quickstartConfig) error {
	var lastErr error
	backoff := cfg.joinRetryBackoff
	for attempt := 0; attempt <= cfg.joinRetries; attempt++ {
		if attempt > 0 {
			warn("    retry %d/%d after %s", attempt, cfg.joinRetries, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff *= 2
		}
		err := joinNode(ctx, c, networkID, nodeID)
		if err == nil {
			return nil
		}
		lastErr = err
		// Don't retry on obvious non-transient errors — saves ~15s per bad
		// request. 400-class errors are typically caller mistakes, not races.
		if isNonTransient(err) {
			return err
		}
	}
	return lastErr
}

func joinNode(ctx context.Context, c *apiClient, networkID, nodeID int64) error {
	resp, err := c.post(ctx, fmt.Sprintf("/networks/fabricx/%d/nodes/%d/join", networkID, nodeID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return errors.New(readErrorBody(resp))
}

// createNamespace POSTs a namespace creation to the FabricX network with
// waitForFinality=true, so the API returns only once the committer has applied
// the tx. The server's error (if any) is surfaced directly — namespace
// failures are the clearest signal of end-to-end unhealthy state.
func createNamespace(ctx context.Context, c *apiClient, networkID int64, cfg quickstartConfig, submitterOrgID int64) error {
	timeoutSeconds := int(cfg.namespaceTimeout / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	body := map[string]any{
		"name":                   cfg.namespaceName,
		"submitterOrgId":         submitterOrgID,
		"waitForFinality":        true,
		"finalityTimeoutSeconds": timeoutSeconds,
	}
	resp, err := c.post(ctx, fmt.Sprintf("/networks/fabricx/%d/namespaces", networkID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}
	return fmt.Errorf("%s", readErrorBody(resp))
}

// startNodeWithRetry issues POST /nodes/{id}/start with exponential backoff.
// The endpoint is idempotent (the service no-ops for already-running
// containers), so retrying is always safe. This exists to recover from the
// Docker Desktop bind-mount race where the initial join's start attempt
// silently fails — by the time we reach the reconcile phase, the apiproxy
// queue has drained and the second attempt succeeds.
func startNodeWithRetry(ctx context.Context, c *apiClient, nodeID int64, cfg quickstartConfig) error {
	var lastErr error
	backoff := cfg.joinRetryBackoff
	for attempt := 0; attempt <= cfg.joinRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff *= 2
		}
		resp, err := c.post(ctx, fmt.Sprintf("/nodes/%d/start", nodeID), nil)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent {
			resp.Body.Close()
			return nil
		}
		lastErr = fmt.Errorf("%s", readErrorBody(resp))
		resp.Body.Close()
		if isNonTransient(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

func isNonTransient(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// 4xx from the API is a caller-side problem (bad IDs, validation) — retry
	// won't help. 5xx and transport errors are worth retrying.
	for _, code := range []string{"HTTP 400", "HTTP 401", "HTTP 403", "HTTP 404", "HTTP 409", "HTTP 422"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

// verifyNodeJoined fetches the node and checks whether its status reflects a
// successful join. The current backend flips status to "joined" even when the
// container start fails, so "joined" alone isn't enough — we also check that
// it is running (or at least not in an error state).
//
// Returns (true, "") when healthy. Returns (false, reason) when the node is
// stuck, which the caller should surface to the operator.
func verifyNodeJoined(ctx context.Context, c *apiClient, nodeID int64) (bool, string) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%d", nodeID))
	if err != nil {
		return false, fmt.Sprintf("status check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("status check HTTP %d", resp.StatusCode)
	}
	var node struct {
		Status   string `json:"status"`
		ErrorMsg string `json:"errorMessage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return false, fmt.Sprintf("decode status: %v", err)
	}
	switch strings.ToUpper(node.Status) {
	case "RUNNING", "JOINED", "READY":
		return true, ""
	case "ERROR", "FAILED":
		detail := node.ErrorMsg
		if detail == "" {
			detail = "(no errorMessage returned)"
		}
		return false, fmt.Sprintf("status=%s: %s", node.Status, detail)
	default:
		return false, fmt.Sprintf("status=%s (expected RUNNING/JOINED)", node.Status)
	}
}

// cleanBundle tears down any prior quickstart bundle so reruns don't hit
// UNIQUE-constraint failures on node_groups.name. Order matters: networks
// first (they reference nodes), then nodes, then node_groups, then orgs.
// The shared Postgres service is left running — it's reused across runs.
func cleanBundle(ctx context.Context, c *apiClient, cfg quickstartConfig) error {
	status("Cleaning any prior bundle named %q", cfg.networkName)

	// Delete networks matching the name.
	if resp, err := c.get(ctx, "/networks/fabricx?limit=500"); err == nil {
		body, _ := readBody(resp)
		var env struct {
			Networks []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"networks"`
		}
		if err := json.Unmarshal(body, &env); err == nil {
			for _, n := range env.Networks {
				if n.Name != cfg.networkName {
					continue
				}
				dresp, derr := c.delete_(ctx, fmt.Sprintf("/networks/fabricx/%d", n.ID))
				if derr != nil {
					warn("  ⚠ delete network #%d: %v", n.ID, derr)
					continue
				}
				dresp.Body.Close()
				done("  → deleted network #%d", n.ID)
			}
		}
	}

	// Delete FabricX nodes (committers + orderer children + any stray).
	if resp, err := c.get(ctx, "/nodes?platform=FABRICX&limit=1000"); err == nil {
		body, _ := readBody(resp)
		var env struct {
			Items []struct {
				ID int64 `json:"id"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &env); err == nil {
			for _, n := range env.Items {
				dresp, derr := c.delete_(ctx, fmt.Sprintf("/nodes/%d", n.ID))
				if derr != nil {
					warn("  ⚠ delete node #%d: %v", n.ID, derr)
					continue
				}
				dresp.Body.Close()
			}
			if len(env.Items) > 0 {
				done("  → deleted %d nodes", len(env.Items))
			}
		}
	}

	// Delete FabricX node_groups.
	if resp, err := c.get(ctx, "/node-groups?limit=500"); err == nil {
		body, _ := readBody(resp)
		var groups []struct {
			ID       int64  `json:"id"`
			Platform string `json:"platform"`
		}
		_ = json.Unmarshal(body, &groups)
		deleted := 0
		for _, g := range groups {
			if !strings.EqualFold(g.Platform, "FABRICX") {
				continue
			}
			dresp, derr := c.delete_(ctx, fmt.Sprintf("/node-groups/%d", g.ID))
			if derr != nil {
				warn("  ⚠ delete node_group #%d: %v", g.ID, derr)
				continue
			}
			dresp.Body.Close()
			deleted++
		}
		if deleted > 0 {
			done("  → deleted %d node_groups", deleted)
		}
	}

	// Delete Party*MSP orgs.
	if resp, err := c.get(ctx, "/organizations?limit=1000"); err == nil {
		body, _ := readBody(resp)
		var env orgListResponse
		_ = json.Unmarshal(body, &env)
		deleted := 0
		for _, o := range env.Items {
			if !strings.HasPrefix(o.MspID, "Party") || !strings.HasSuffix(o.MspID, "MSP") {
				continue
			}
			dresp, derr := c.delete_(ctx, fmt.Sprintf("/organizations/%d", o.ID))
			if derr != nil {
				warn("  ⚠ delete org #%d: %v", o.ID, derr)
				continue
			}
			dresp.Body.Close()
			deleted++
		}
		if deleted > 0 {
			done("  → deleted %d orgs", deleted)
		}
	}

	// Purge on-disk bind-mount state. API delete only drops DB rows — it
	// leaves per-node data/ directories behind. On a rerun, the old TLS
	// cert survives; fresh Init() generates new DB keys; genesis is built
	// from new keys; orderer containers start with a cert that doesn't
	// match the shared config and crash. See orderergroup.go ensureMaterials
	// drift-detection for the matching backend-side fix.
	if cfg.dataPath != "" {
		for _, sub := range []string{"fabricx-orderers", "fabricx-committers"} {
			dir := cfg.dataPath + "/" + sub
			if _, err := os.Stat(dir); err == nil {
				if err := os.RemoveAll(dir); err != nil {
					warn("  ⚠ purge %s: %v", dir, err)
					continue
				}
				done("  → purged %s", dir)
			}
		}
	}

	return nil
}
