package besu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// createVolume creates a Docker volume if it doesn't exist
func (b *LocalBesu) createVolume(ctx context.Context, cli *client.Client, name string) error {
	// Check if volume exists
	volumes, err := cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	for _, vol := range volumes.Volumes {
		if vol.Name == name {
			return nil // Volume already exists
		}
	}

	// Create volume
	_, err = cli.VolumeCreate(ctx, volume.CreateOptions{
		Name:   name,
		Driver: "local",
	})
	if err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	return nil
}

// startDocker starts the besu node in a docker container
func (b *LocalBesu) startDocker(env map[string]string, dataDir, configDir string) (*StartDockerResponse, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	// Write genesis file to config directory
	genesisPath := filepath.Join(configDir, "genesis.json")
	if err := os.WriteFile(genesisPath, []byte(b.opts.GenesisFile), 0644); err != nil {
		return nil, fmt.Errorf("failed to write genesis file: %w", err)
	}

	keyPath := filepath.Join(configDir, "key")
	if err := os.WriteFile(keyPath, []byte(b.opts.NodePrivateKey), 0644); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	// Prepare container configuration
	containerName := b.getContainerName()
	imageName := fmt.Sprintf("hyperledger/besu:%s", b.opts.Version)

	// Create port bindings
	portBindings := nat.PortMap{
		nat.Port(fmt.Sprintf("%s/tcp", b.opts.RPCPort)): []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: b.opts.RPCPort},
		},
		nat.Port(fmt.Sprintf("%s/tcp", b.opts.P2PPort)): []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: b.opts.P2PPort},
		},
		nat.Port(fmt.Sprintf("%s/udp", b.opts.P2PPort)): []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: b.opts.P2PPort},
		},
	}

	// Create container config
	config := &container.Config{
		Image:        imageName,
		Cmd:          b.buildDockerBesuCommand("/opt/besu/data", "/opt/besu/config"),
		Env:          formatEnvForDocker(env),
		ExposedPorts: nat.PortSet{},
	}

	// Add bootnodes if specified
	if len(b.opts.BootNodes) > 0 {
		config.Cmd = append(config.Cmd, fmt.Sprintf("--bootnodes=%s", strings.Join(b.opts.BootNodes, ",")))
	}

	// Add exposed ports
	for port := range portBindings {
		config.ExposedPorts[port] = struct{}{}
	}

	// Create host config with bind mounts instead of volumes
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: dataDir,
				Target: "/opt/besu/data",
			},
			{
				Type:   mount.TypeBind,
				Source: configDir,
				Target: "/opt/besu/config",
			},
		},
	}

	// Remove existing container if it exists
	if err := b.removeExistingContainer(ctx, cli, containerName); err != nil {
		return nil, err
	}
	// Pull the image
	_, err = cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	// Create container
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return &StartDockerResponse{
		Mode:          "docker",
		ContainerName: containerName,
	}, nil
}

// stopDocker stops the besu docker container
func (b *LocalBesu) stopDocker() error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	return b.removeExistingContainer(ctx, cli, b.getContainerName())
}

// removeExistingContainer removes an existing container if it exists
func (b *LocalBesu) removeExistingContainer(ctx context.Context, cli *client.Client, containerName string) error {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		for _, name := range c.Names {
			if name == "/"+containerName {
				if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{
					Force: true,
				}); err != nil {
					return fmt.Errorf("failed to remove existing container: %w", err)
				}
				break
			}
		}
	}

	return nil
}

// getContainerName returns the docker container name
func (b *LocalBesu) getContainerName() string {
	return fmt.Sprintf("besu-%s", strings.ReplaceAll(strings.ToLower(b.opts.ID), " ", "-"))
}

// formatEnvForDocker formats environment variables for docker
func formatEnvForDocker(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// buildBesuCommand builds the command arguments for Besu
func (b *LocalBesu) buildDockerBesuCommand(dataPath, configPath string) []string {
	cmd := []string{
		"besu",
		fmt.Sprintf("--network-id=%d", b.opts.ChainID),
		fmt.Sprintf("--data-path=%s", dataPath),
		fmt.Sprintf("--genesis-file=%s", filepath.Join(configPath, "genesis.json")),
		"--rpc-http-enabled",
		fmt.Sprintf("--rpc-http-port=%s", b.opts.RPCPort),
		fmt.Sprintf("--p2p-port=%s", b.opts.P2PPort),
		"--rpc-http-api=ADMIN,ETH,NET,PERM,QBFT,WEB3,TXPOOL",
		"--host-allowlist=*",
		"--miner-enabled",
		fmt.Sprintf("--miner-coinbase=%s", b.opts.MinerAddress),
		"--min-gas-price=1000000000",
		"--rpc-http-cors-origins=all",
		fmt.Sprintf("--node-private-key-file=%s", filepath.Join(configPath, "key")),
		fmt.Sprintf("--p2p-host=%s", b.opts.ListenAddress),
		"--rpc-http-host=0.0.0.0",
		"--discovery-enabled=true",
		"--sync-mode=FULL",
		"--revert-reason-enabled=true",
		"--validator-priority-enabled=true",
	}

	// Add bootnodes if specified
	if len(b.opts.BootNodes) > 0 {
		cmd = append(cmd, fmt.Sprintf("--bootnodes=%s", strings.Join(b.opts.BootNodes, ",")))
	}

	return cmd
}
