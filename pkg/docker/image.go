package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// PullImageIfNeeded pulls the Docker image if not present locally.
// It tries docker CLI first (to use credential helpers like ecr-login),
// then falls back to Docker API if CLI fails.
func PullImageIfNeeded(ctx context.Context, cli *client.Client, imageName string) error {
	// Check if image exists locally
	_, _, err := cli.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		return nil // Image exists locally
	}

	// Try docker pull CLI first - this properly uses credential helpers from ~/.docker/config.json
	// (e.g., docker-credential-ecr-login for ECR authentication)
	cmd := exec.CommandContext(ctx, "docker", "pull", imageName)
	output, err := cmd.CombinedOutput()
	if err == nil {
		// CLI pull succeeded
		return nil
	}

	// CLI pull failed, fall back to Docker API
	// Note: Docker API doesn't automatically use credential helpers, but we try anyway
	reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		// Both methods failed - return detailed error
		return fmt.Errorf("failed to pull image %s: docker CLI failed (%s), docker API failed (%v)", imageName, string(output), err)
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("error while pulling image via API: %v", err)
	}
	return nil
}
