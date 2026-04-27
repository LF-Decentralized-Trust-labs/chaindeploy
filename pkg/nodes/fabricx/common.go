package fabricx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/docker"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// containerCreateMu serializes ContainerCreate calls globally to prevent the
// macOS Docker Desktop apiproxy from saturating its bind-mount symlink
// evaluation queue. When ~20+ concurrent ContainerCreate requests arrive,
// apiproxy starts returning "bind source path does not exist" for paths that
// actually exist. Serializing creates keeps the queue bounded.
var containerCreateMu sync.Mutex

const (
	// v1.0.0-alpha images come from ghcr.io (not docker hub). They are
	// schema-backward-compatible with v0.0.24/v0.1.9 configs — existing
	// chaindeploy-generated YAML loads without modification. The only
	// user-visible break is the committer's CLI: `start-<service>` was
	// renamed to `start <service>` (subcommand) in #491 — handled in
	// roles.go.
	DefaultOrdererImage   = "ghcr.io/hyperledger/fabric-x-orderer"
	DefaultOrdererVersion = "1.0.0-alpha"

	DefaultCommitterImage   = "ghcr.io/hyperledger/fabric-x-committer"
	DefaultCommitterVersion = "1.0.0-alpha"

	DefaultPostgresImage   = "postgres"
	DefaultPostgresVersion = "16-alpine"

	// localDevHost is the hostname used when CHAINLAUNCH_FABRICX_LOCAL_DEV
	// is set. Docker Desktop (macOS/Windows) resolves host.docker.internal
	// to the host — the only reliable way containers can reach each other's
	// published ports, since numeric IP dials bypass /etc/hosts.
	localDevHost = "host.docker.internal"
)

// slugify converts a name to a Docker-safe slug
func slugify(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

// containerNamePrefix returns a consistent container name prefix
func containerNamePrefix(mspID, name string) string {
	return fmt.Sprintf("fabricx-%s-%s", strings.ToLower(mspID), slugify(name))
}

// writeTemplate executes a Go template and writes the result to a file
func writeTemplate(tmplStr, path string, data interface{}) error {
	tmpl, err := template.New("config").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// writeMSP writes MSP directory structure
func writeMSP(mspDir, signCert, signKey, caCert, tlsCACert string) error {
	dirs := []string{
		filepath.Join(mspDir, "signcerts"),
		filepath.Join(mspDir, "keystore"),
		filepath.Join(mspDir, "cacerts"),
		filepath.Join(mspDir, "tlscacerts"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create MSP directory %s: %w", d, err)
		}
	}

	files := map[string]string{
		filepath.Join(mspDir, "signcerts", "cert.pem"):    signCert,
		filepath.Join(mspDir, "keystore", "priv_sk"):       signKey,
		filepath.Join(mspDir, "cacerts", "ca.pem"):        caCert,
		filepath.Join(mspDir, "tlscacerts", "ca.pem"):     tlsCACert,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	// Write NodeOU config.yaml
	configYAML := `NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/ca.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/ca.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/ca.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/ca.pem
    OrganizationalUnitIdentifier: orderer
`
	return os.WriteFile(filepath.Join(mspDir, "config.yaml"), []byte(configYAML), 0644)
}

// writeTLS writes TLS cert and key
func writeTLS(tlsDir, tlsCert, tlsKey, tlsCACert string) error {
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		return fmt.Errorf("failed to create TLS directory: %w", err)
	}
	files := map[string]string{
		filepath.Join(tlsDir, "server.crt"): tlsCert,
		filepath.Join(tlsDir, "server.key"): tlsKey,
		filepath.Join(tlsDir, "ca.crt"):     tlsCACert,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}
	return nil
}

// startContainer pulls an image if needed and starts a Docker container.
// extraHosts entries are added to /etc/hosts (e.g. "192.168.1.133:host-gateway"
// so containers can route the configured external IP back to the host on macOS
// Docker Desktop, where loopback to a bound host port is otherwise unreachable
// from inside a container).
func startContainer(
	ctx context.Context,
	log *logger.Logger,
	imageName string,
	containerName string,
	cmd []string,
	env map[string]string,
	portBindings map[nat.Port][]nat.PortBinding,
	mounts []mount.Mount,
	networkName string,
	extraHosts []string,
) error {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	if err := docker.PullImageIfNeeded(ctx, cli, imageName); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}

	// Remove existing container if present. Log the result so we can tell
	// whether a downstream "name already in use" is a genuine race or a
	// silent remove failure.
	if rmErr := cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true}); rmErr != nil && !dockerclient.IsErrNotFound(rmErr) {
		log.Warn("pre-create ContainerRemove failed; will retry on name conflict",
			"container", containerName, "err", rmErr.Error())
	}

	envSlice := mapToEnvSlice(env)

	exposedPorts := make(map[nat.Port]struct{})
	for port := range portBindings {
		exposedPorts[port] = struct{}{}
	}

	containerConfig := &container.Config{
		Image:        imageName,
		Cmd:          cmd,
		Env:          envSlice,
		ExposedPorts: exposedPorts,
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Mounts:       mounts,
		ExtraHosts:   extraHosts,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyAlways,
		},
	}

	if networkName != "" {
		hostConfig.NetworkMode = container.NetworkMode(networkName)
	}

	// On macOS Docker Desktop the daemon's apiproxy serializes bind-mount path
	// resolution (it calls evaluateSymlinks for every Source on the host).
	// Under load it takes ~60-90s to drain the queue, during which time fresh
	// ContainerCreate calls return "bind source path does not exist" — even
	// though the path is on disk and visible to other Docker clients.
	//
	// Strategy: ensure the directories really exist, then back off slowly
	// (2s, 4s, 8s, 16s, 30s, 30s, ...) to avoid piling more requests onto the
	// already-saturated apiproxy queue. Total budget ~3min, after which we
	// give up.
	ensureMountSources := func() {
		for _, m := range mounts {
			if m.Type == mount.TypeBind && m.Source != "" {
				_ = os.MkdirAll(m.Source, 0755)
				_, _ = os.Stat(m.Source)
				// fsync the directory so Docker Desktop's apiproxy sees a
				// fully-committed inode when it calls stat(). Without this,
				// macOS sometimes returns EPERM on a brand-new directory
				// whose metadata hasn't been flushed yet.
				if d, err := os.Open(m.Source); err == nil {
					_ = d.Sync()
					_ = d.Close()
				}
			}
		}
	}
	ensureMountSources()

	// Serialize ContainerCreate globally on macOS Docker Desktop. This lets
	// apiproxy resolve bind-mount sources sequentially instead of drowning in
	// a saturated queue under parallel E2E load.
	containerCreateMu.Lock()
	defer containerCreateMu.Unlock()

	var resp container.CreateResponse
	var createErr error
	nameConflictRetried := false
	backoffs := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}
	for attempt := 0; attempt <= len(backoffs); attempt++ {
		resp, createErr = cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
		if createErr == nil {
			break
		}
		// A leftover container with the same name must be force-removed before
		// ContainerCreate can reuse the name. The pre-create remove above may
		// fail silently (e.g. apiproxy transient) or a race may create a
		// same-named container between remove and create. Retry once after an
		// explicit force-remove.
		if strings.Contains(createErr.Error(), "is already in use") && !nameConflictRetried {
			nameConflictRetried = true
			log.Warn("container name conflict; force-removing and retrying",
				"container", containerName, "err", createErr.Error())
			if rmErr := cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true}); rmErr != nil && !dockerclient.IsErrNotFound(rmErr) {
				return fmt.Errorf("failed to remove conflicting container %s: %w", containerName, rmErr)
			}
			continue
		}
		// Transient macOS Docker Desktop bind-mount errors we should retry:
		//  - "bind source path does not exist" (apiproxy queue saturated)
		//  - "operation not permitted" on stat (fsnotify race while we still
		//    have file handles open, or apiproxy file-descriptor pressure)
		errStr := createErr.Error()
		transient := strings.Contains(errStr, "bind source path does not exist") ||
			(strings.Contains(errStr, "invalid mount config for type \"bind\"") &&
				strings.Contains(errStr, "operation not permitted"))
		if !transient {
			break
		}
		if attempt == len(backoffs) {
			break
		}
		wait := backoffs[attempt]
		log.Debug("docker apiproxy slow resolving bind mount, backing off",
			"container", containerName, "attempt", attempt+1, "waitFor", wait, "err", createErr.Error())
		time.Sleep(wait)
		ensureMountSources()
	}
	if createErr != nil {
		return fmt.Errorf("failed to create container %s: %w", containerName, createErr)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerName, err)
	}

	log.Info("Started container", "name", containerName, "id", resp.ID[:12])
	return nil
}

// stopContainer stops and removes a Docker container
func stopContainer(ctx context.Context, containerName string) error {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	timeout := 10
	_ = cli.ContainerStop(ctx, containerName, container.StopOptions{Timeout: &timeout})
	_ = cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})
	return nil
}

// isContainerRunning checks if a Docker container is running
func isContainerRunning(ctx context.Context, containerName string) (bool, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	info, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return false, nil // container doesn't exist
	}
	return info.State.Running, nil
}

// waitForContainer waits for a container to be running
func waitForContainer(ctx context.Context, containerName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		running, err := isContainerRunning(ctx, containerName)
		if err != nil {
			return err
		}
		if running {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("container %s did not start within %s", containerName, timeout)
}

func mapToEnvSlice(m map[string]string) []string {
	var env []string
	for k, v := range m {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func portBinding(hostPort int) map[nat.Port][]nat.PortBinding {
	port := nat.Port(fmt.Sprintf("%d/tcp", hostPort))
	return map[nat.Port][]nat.PortBinding{
		port: {{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", hostPort)}},
	}
}

// createDockerNetwork creates a Docker bridge network if it doesn't exist
func createDockerNetwork(ctx context.Context, networkName string) error {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	// Check if network already exists
	networks, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}
	for _, n := range networks {
		if n.Name == networkName {
			return nil
		}
	}

	_, err = cli.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w", networkName, err)
	}
	return nil
}
