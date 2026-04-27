//go:build integration

package postgres_test

// Docker-gated integration test for pkg/services/postgres. Covers the
// full lifecycle used by the node_groups coordinator:
//
//   Start → IsRunning=true → WaitReady (pg_isready) → Stop → IsRunning=false
//
// Run with: go test -tags=integration ./pkg/services/postgres/... -run TestPostgresLifecycle -v
//
// Skipped automatically when the docker daemon is unreachable so CI
// environments without docker don't report false failures.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/services/postgres"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
)

func requireDocker(t *testing.T) *dockerclient.Client {
	t.Helper()
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skipf("docker client unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}
	return cli
}

func ensureNetwork(t *testing.T, cli *dockerclient.Client, name string) {
	t.Helper()
	ctx := context.Background()
	nets, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		t.Fatalf("network list: %v", err)
	}
	for _, n := range nets {
		if n.Name == name {
			return
		}
	}
	if _, err := cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge"}); err != nil {
		t.Fatalf("network create %s: %v", name, err)
	}
}

func TestPostgresLifecycle(t *testing.T) {
	cli := requireDocker(t)
	defer cli.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	networkName := "chainlaunch-pg-it-" + suffix
	containerName := "chainlaunch-pg-it-" + suffix

	ensureNetwork(t, cli, networkName)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = postgres.Stop(ctx, containerName)
		_ = cli.NetworkRemove(ctx, networkName)
	})

	log := logger.NewDefault()

	// Pre-condition: container must not exist.
	running, err := postgres.IsRunning(context.Background(), containerName)
	if err != nil {
		t.Fatalf("IsRunning pre-start: %v", err)
	}
	if running {
		t.Fatalf("container %s unexpectedly running before Start", containerName)
	}

	// Start — pulls image, starts container, waits for pg_isready.
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer startCancel()
	id, err := postgres.Start(startCtx, log, postgres.Config{
		ContainerName: containerName,
		NetworkName:   networkName,
		DB:            "chainlaunch_it",
		User:          "chainlaunch",
		Password:      "it-password",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if id == "" {
		t.Fatal("Start returned empty container ID")
	}

	// IsRunning should now be true.
	running, err = postgres.IsRunning(context.Background(), containerName)
	if err != nil {
		t.Fatalf("IsRunning post-start: %v", err)
	}
	if !running {
		t.Fatal("expected container to be running after Start")
	}

	// WaitReady idempotently — Start already waited, so this must return
	// immediately on a ready server.
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer readyCancel()
	if err := postgres.WaitReady(readyCtx, containerName, "chainlaunch", "chainlaunch_it", 10*time.Second); err != nil {
		t.Fatalf("WaitReady on ready server: %v", err)
	}

	// Stop — must tear down so IsRunning flips back to false.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()
	if err := postgres.Stop(stopCtx, containerName); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	running, err = postgres.IsRunning(context.Background(), containerName)
	if err != nil {
		t.Fatalf("IsRunning post-stop: %v", err)
	}
	if running {
		t.Fatal("expected container to be absent after Stop, IsRunning=true")
	}

	// Stop is idempotent — second call on a missing container must not error.
	if err := postgres.Stop(context.Background(), containerName); err != nil {
		t.Fatalf("Stop (second call): %v", err)
	}
}

func TestPostgresStart_RejectsInvalidConfig(t *testing.T) {
	// No docker required — these all fail at the config-validation
	// gate before any docker call.
	log := logger.NewDefault()
	ctx := context.Background()

	cases := []struct {
		name string
		cfg  postgres.Config
		want string
	}{
		{"missing container name", postgres.Config{NetworkName: "n", Password: "p"}, "ContainerName"},
		{"missing network name", postgres.Config{ContainerName: "c", Password: "p"}, "NetworkName"},
		{"missing password", postgres.Config{ContainerName: "c", NetworkName: "n"}, "Password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := postgres.Start(ctx, log, tc.cfg)
			if err == nil {
				t.Fatal("expected error for invalid config")
			}
			if !containsSubstr(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
