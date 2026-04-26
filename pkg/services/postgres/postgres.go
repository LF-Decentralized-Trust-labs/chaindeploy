// Package postgres implements the managed PostgreSQL service used by
// FabricX committer groups (and, in the future, any other subsystem that
// needs a first-class managed postgres).
//
// The service runs a single container on the node_group's bridge network
// so siblings can dial it by container name. It is provisioned per
// node_group and is distinct from the node_groups lifecycle: callers
// start postgres first, wait for pg_isready, then start committer roles.
//
// WAL-G integration is intentionally minimal in this first cut — the
// ADR 0001 plan parks the WAL archiving sidecar work until a
// services-level scheduler lands. For now this package:
//   - pulls the upstream postgres image (matching the historical default)
//   - runs it on a caller-provided network with POSTGRES_DB/USER/PASSWORD
//   - publishes the configured host port
//   - exposes Start/Stop/IsRunning so the node_groups coordinator can
//     treat it like any other lifecycle unit
package postgres

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/docker"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	// DefaultImage matches the historical DefaultPostgresImage used by the
	// embedded committer postgres, so migrating node_groups onto the
	// managed service doesn't change the runtime.
	DefaultImage   = "postgres"
	DefaultVersion = "16-alpine"

	// ReadyTimeout caps how long Start waits for pg_isready. Matches the
	// 3s sleep the monolithic Committer.Start used plus headroom.
	ReadyTimeout = 30 * time.Second
)

// Config is the minimum needed to materialize a postgres container.
// Everything is populated by the node_groups coordinator from the
// services row and the owning group's deployment config.
type Config struct {
	// ContainerName must be unique and stable across restarts.
	ContainerName string
	// NetworkName is the docker bridge network shared with siblings.
	// Required — siblings dial postgres by container name on this net.
	NetworkName string
	// HostPort is the published port on the host. Zero means "do not
	// publish" (container is only reachable on NetworkName).
	HostPort int
	// Version is the postgres image tag. Defaults to DefaultVersion.
	Version string
	// Credentials and database. POSTGRES_PASSWORD is required by the
	// upstream image or it refuses to start.
	DB       string
	User     string
	Password string
	// ExtraEnv optionally adds env vars (e.g. WAL-G creds once the
	// archiving sidecar ships). Nil is fine.
	ExtraEnv map[string]string
}

// Start materializes the postgres container, replacing any existing one
// with the same name, and waits until pg_isready reports ready. Returns
// the container ID on success.
func Start(ctx context.Context, log *logger.Logger, cfg Config) (string, error) {
	if cfg.ContainerName == "" {
		return "", fmt.Errorf("postgres: ContainerName is required")
	}
	if cfg.NetworkName == "" {
		return "", fmt.Errorf("postgres: NetworkName is required")
	}
	if cfg.Password == "" {
		return "", fmt.Errorf("postgres: Password is required (upstream image refuses empty passwords)")
	}

	version := cfg.Version
	if version == "" {
		version = DefaultVersion
	}
	imageName := fmt.Sprintf("%s:%s", DefaultImage, version)

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return "", fmt.Errorf("postgres: docker client: %w", err)
	}
	defer cli.Close()

	if err := docker.PullImageIfNeeded(ctx, cli, imageName); err != nil {
		return "", fmt.Errorf("postgres: pull image %s: %w", imageName, err)
	}

	// Ensure the target bridge network exists. Creating it here (rather
	// than forcing every caller to) keeps the node_groups coordinator
	// free of docker plumbing and matches the semantics of the fabricx
	// per-group network helper.
	if err := ensureNetwork(ctx, cli, cfg.NetworkName); err != nil {
		return "", fmt.Errorf("postgres: %w", err)
	}

	_ = cli.ContainerRemove(ctx, cfg.ContainerName, container.RemoveOptions{Force: true})

	env := []string{
		fmt.Sprintf("POSTGRES_DB=%s", cfg.DB),
		fmt.Sprintf("POSTGRES_USER=%s", cfg.User),
		fmt.Sprintf("POSTGRES_PASSWORD=%s", cfg.Password),
	}
	for k, v := range cfg.ExtraEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	containerPort := nat.Port("5432/tcp")
	exposed := map[nat.Port]struct{}{containerPort: {}}
	var portBindings map[nat.Port][]nat.PortBinding
	if cfg.HostPort > 0 {
		portBindings = map[nat.Port][]nat.PortBinding{
			containerPort: {{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", cfg.HostPort)}},
		}
	}

	containerConfig := &container.Config{
		Image:        imageName,
		Env:          env,
		ExposedPorts: exposed,
	}
	hostConfig := &container.HostConfig{
		PortBindings:  portBindings,
		NetworkMode:   container.NetworkMode(cfg.NetworkName),
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
	}

	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, cfg.ContainerName)
	if err != nil {
		return "", fmt.Errorf("postgres: create container: %w", err)
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("postgres: start container: %w", err)
	}

	log.Info("Started managed postgres", "container", cfg.ContainerName, "id", resp.ID[:12])

	if err := WaitReady(ctx, cfg.ContainerName, cfg.User, cfg.DB, ReadyTimeout); err != nil {
		return resp.ID, fmt.Errorf("postgres: %w", err)
	}
	return resp.ID, nil
}

// Stop stops and removes the postgres container. Safe to call when the
// container does not exist.
func Stop(ctx context.Context, containerName string) error {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("postgres: docker client: %w", err)
	}
	defer cli.Close()

	timeout := 10
	_ = cli.ContainerStop(ctx, containerName, container.StopOptions{Timeout: &timeout})
	_ = cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})
	return nil
}

// IsRunning reports docker-level running state for the postgres
// container. A false result with nil error means the container does not
// exist (not an error — the coordinator uses this to decide whether to
// start it).
func IsRunning(ctx context.Context, containerName string) (bool, error) {
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
		return false, nil
	}
	return info.State.Running, nil
}

// ConnectNetwork attaches an already-running postgres container to an
// additional bridge network so a new committer group can reach it by
// container name. Idempotent: if the container is already on the
// network, returns nil. Creates the network if it does not exist so the
// caller (node_groups coordinator) does not need to race with fabricx
// on who creates the per-group bridge.
//
// This is what makes one postgres service shareable across N committer
// groups without restarting postgres — the container stays up and just
// grows a new network membership per consumer.
func ConnectNetwork(ctx context.Context, containerName, networkName string) error {
	if containerName == "" {
		return fmt.Errorf("postgres: ContainerName is required")
	}
	if networkName == "" {
		return fmt.Errorf("postgres: NetworkName is required")
	}

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("postgres: docker client: %w", err)
	}
	defer cli.Close()

	if err := ensureNetwork(ctx, cli, networkName); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	info, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return fmt.Errorf("postgres: inspect %s: %w", containerName, err)
	}
	if info.NetworkSettings != nil {
		if _, already := info.NetworkSettings.Networks[networkName]; already {
			return nil
		}
	}

	if err := cli.NetworkConnect(ctx, networkName, containerName, nil); err != nil {
		return fmt.Errorf("postgres: network connect %s → %s: %w", containerName, networkName, err)
	}
	return nil
}

// DisconnectNetwork detaches the postgres container from a bridge
// network when a committer group is torn down. Best-effort and
// idempotent — if the container is not on the network (or does not
// exist), returns nil. The container is left running so other consumers
// can keep using it; fully stopping it is an explicit /services/{id}/stop
// operation.
func DisconnectNetwork(ctx context.Context, containerName, networkName string) error {
	if containerName == "" || networkName == "" {
		return nil
	}

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("postgres: docker client: %w", err)
	}
	defer cli.Close()

	info, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return nil
	}
	if info.NetworkSettings == nil {
		return nil
	}
	if _, attached := info.NetworkSettings.Networks[networkName]; !attached {
		return nil
	}
	if err := cli.NetworkDisconnect(ctx, networkName, containerName, true); err != nil {
		return fmt.Errorf("postgres: network disconnect %s ← %s: %w", containerName, networkName, err)
	}
	return nil
}

// ensureNetwork is a no-op if networkName already exists, otherwise
// creates a bridge network with that name. Idempotent — safe to call
// across restarts and alongside fabricx's per-group network helper.
func ensureNetwork(ctx context.Context, cli *dockerclient.Client, networkName string) error {
	nets, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("list networks: %w", err)
	}
	for _, n := range nets {
		if n.Name == networkName {
			return nil
		}
	}
	if _, err := cli.NetworkCreate(ctx, networkName, network.CreateOptions{Driver: "bridge"}); err != nil {
		return fmt.Errorf("create network %s: %w", networkName, err)
	}
	return nil
}

// DatabaseSpec is one per-consumer database + role to provision inside a
// shared postgres container. Used by CreateDatabases so one postgres can
// host N tenants, each with its own DB and login role.
type DatabaseSpec struct {
	DB       string
	User     string
	Password string
}

// CreateDatabases ensures each (DB, User, Password) exists inside the
// running postgres container, idempotently. Safe to call repeatedly —
// existing roles keep their passwords unless explicitly updated, and
// existing databases are left alone. Runs as the caller-supplied admin
// user (the POSTGRES_USER baked into the container).
//
// This is what lets the FabricX quickstart run a single postgres and
// give each committer its own database + role without N containers.
func CreateDatabases(ctx context.Context, containerName, adminUser string, specs []DatabaseSpec) error {
	if containerName == "" {
		return fmt.Errorf("postgres: ContainerName is required")
	}
	if adminUser == "" {
		return fmt.Errorf("postgres: adminUser is required")
	}
	if len(specs) == 0 {
		return nil
	}

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("postgres: docker client: %w", err)
	}
	defer cli.Close()

	for _, s := range specs {
		if s.DB == "" || s.User == "" || s.Password == "" {
			return fmt.Errorf("postgres: CreateDatabases: db/user/password required (got %+v)", s)
		}
		// Two-stage to keep it idempotent: role first (DO block swallows
		// duplicate_object), then database (query pg_database and skip if
		// present — CREATE DATABASE cannot run inside a DO block).
		roleSQL := fmt.Sprintf(
			`DO $do$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN CREATE ROLE "%s" LOGIN PASSWORD '%s'; ELSE ALTER ROLE "%s" WITH LOGIN PASSWORD '%s'; END IF; END $do$;`,
			sqlIdent(s.User), sqlIdent(s.User), sqlLit(s.Password), sqlIdent(s.User), sqlLit(s.Password),
		)
		if err := execPsql(ctx, cli, containerName, adminUser, roleSQL); err != nil {
			return fmt.Errorf("postgres: create/alter role %s: %w", s.User, err)
		}

		existsSQL := fmt.Sprintf(`SELECT 1 FROM pg_database WHERE datname = '%s'`, sqlIdent(s.DB))
		out, err := execPsqlCapture(ctx, cli, containerName, adminUser, existsSQL)
		if err != nil {
			return fmt.Errorf("postgres: check database %s: %w", s.DB, err)
		}
		if !strings.Contains(out, "(1 row)") {
			createSQL := fmt.Sprintf(`CREATE DATABASE "%s" OWNER "%s"`, sqlIdent(s.DB), sqlIdent(s.User))
			if err := execPsql(ctx, cli, containerName, adminUser, createSQL); err != nil {
				return fmt.Errorf("postgres: create database %s: %w", s.DB, err)
			}
		}
		grantSQL := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s"`, sqlIdent(s.DB), sqlIdent(s.User))
		if err := execPsql(ctx, cli, containerName, adminUser, grantSQL); err != nil {
			return fmt.Errorf("postgres: grant on %s: %w", s.DB, err)
		}
	}
	return nil
}

// execPsql runs a single SQL statement as adminUser inside the container
// and returns an error if the psql exec exits non-zero.
func execPsql(ctx context.Context, cli *dockerclient.Client, containerName, adminUser, sqlStmt string) error {
	exec, err := cli.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd:          []string{"psql", "-U", adminUser, "-v", "ON_ERROR_STOP=1", "-c", sqlStmt},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return err
	}
	resp, err := cli.ContainerExecAttach(ctx, exec.ID, container.ExecStartOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()
	_, _ = io.Copy(io.Discard, resp.Reader)
	for i := 0; i < 50; i++ {
		inspect, ierr := cli.ContainerExecInspect(ctx, exec.ID)
		if ierr == nil && !inspect.Running {
			if inspect.ExitCode != 0 {
				return fmt.Errorf("psql exited %d for: %s", inspect.ExitCode, sqlStmt)
			}
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("psql timed out for: %s", sqlStmt)
}

// execPsqlCapture is like execPsql but returns stdout+stderr as a string
// for callers that need to inspect output (e.g. existence checks).
func execPsqlCapture(ctx context.Context, cli *dockerclient.Client, containerName, adminUser, sqlStmt string) (string, error) {
	exec, err := cli.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd:          []string{"psql", "-U", adminUser, "-tAc", sqlStmt},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", err
	}
	resp, err := cli.ContainerExecAttach(ctx, exec.ID, container.ExecStartOptions{})
	if err != nil {
		return "", err
	}
	defer resp.Close()
	var buf strings.Builder
	_, _ = io.Copy(&buf, resp.Reader)
	for i := 0; i < 50; i++ {
		inspect, ierr := cli.ContainerExecInspect(ctx, exec.ID)
		if ierr == nil && !inspect.Running {
			if inspect.ExitCode != 0 {
				return buf.String(), fmt.Errorf("psql exited %d", inspect.ExitCode)
			}
			return buf.String(), nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return buf.String(), fmt.Errorf("psql capture timed out")
}

// sqlIdent strips quotes/backticks/semicolons from an identifier fragment
// before interpolation. Identifiers are additionally double-quoted at the
// call site.
func sqlIdent(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '"', '`', '\'', ';', '\\', 0:
			return -1
		}
		return r
	}, s)
}

// sqlLit escapes single quotes for inclusion inside a SQL literal.
func sqlLit(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// Logs returns the last `tail` lines of stdout+stderr from the postgres
// container. Returns an empty string with nil error if the container is
// missing (the UI can render a friendly "not running" state). Non-zero
// tail caps the fetch so we don't page megabytes into the response.
func Logs(ctx context.Context, containerName string, tail int) (string, error) {
	if containerName == "" {
		return "", fmt.Errorf("postgres: ContainerName is required")
	}

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return "", fmt.Errorf("postgres: docker client: %w", err)
	}
	defer cli.Close()

	if _, err := cli.ContainerInspect(ctx, containerName); err != nil {
		// Container doesn't exist — treat as empty so the UI can render
		// a friendly "service has never been started" state.
		return "", nil
	}

	tailStr := "200"
	if tail > 0 {
		tailStr = fmt.Sprintf("%d", tail)
	}
	rc, err := cli.ContainerLogs(ctx, containerName, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tailStr,
		Timestamps: false,
	})
	if err != nil {
		return "", fmt.Errorf("postgres: container logs: %w", err)
	}
	defer rc.Close()

	// Docker multiplexes stdout/stderr into an 8-byte-header stream
	// unless TTY=true. Postgres containers do not use TTY, so demux.
	return demuxDockerStream(rc)
}

// demuxDockerStream reads docker's multiplexed log stream and returns
// stdout+stderr concatenated as plain text. Header is 8 bytes:
//
//	[stream_type, 0, 0, 0, size_be_uint32]
//
// See: https://pkg.go.dev/github.com/docker/docker/client#Client.ContainerLogs
func demuxDockerStream(rc io.Reader) (string, error) {
	var out strings.Builder
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(rc, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return out.String(), nil
			}
			return out.String(), err
		}
		size := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])
		if size <= 0 {
			continue
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(rc, buf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				out.Write(buf)
				return out.String(), nil
			}
			return out.String(), err
		}
		out.Write(buf)
	}
}

// WaitReady polls pg_isready inside the container until it reports ready
// or the timeout elapses. Committer siblings cannot connect until this
// passes — a running container is not the same as a ready server.
func WaitReady(ctx context.Context, containerName, user, db string, timeout time.Duration) error {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer cli.Close()

	deadline := time.Now().Add(timeout)
	cmd := []string{"pg_isready", "-U", user, "-d", db}
	for time.Now().Before(deadline) {
		exec, err := cli.ContainerExecCreate(ctx, containerName, container.ExecOptions{
			Cmd:          cmd,
			AttachStdout: false,
			AttachStderr: false,
		})
		if err == nil {
			if err := cli.ContainerExecStart(ctx, exec.ID, container.ExecStartOptions{}); err == nil {
				// Brief poll loop for exit code.
				for i := 0; i < 10; i++ {
					inspect, ierr := cli.ContainerExecInspect(ctx, exec.ID)
					if ierr == nil && !inspect.Running {
						if inspect.ExitCode == 0 {
							return nil
						}
						break
					}
					time.Sleep(200 * time.Millisecond)
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("postgres %s not ready within %s", containerName, timeout)
}
