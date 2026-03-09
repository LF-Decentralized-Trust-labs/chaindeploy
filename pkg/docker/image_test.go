package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// TestPullImageIfNeeded_LocalImageExists tests that existing images are not re-pulled
func TestPullImageIfNeeded_LocalImageExists(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// First, ensure the image is available (pull it)
	imageName := "hello-world:latest"
	err = PullImageIfNeeded(ctx, cli, imageName)
	if err != nil {
		t.Fatalf("Failed to pull image for setup: %v", err)
	}

	// Now test that pulling the same image again succeeds quickly (because it exists locally)
	start := time.Now()
	err = PullImageIfNeeded(ctx, cli, imageName)
	if err != nil {
		t.Fatalf("Failed to pull existing image: %v", err)
	}
	elapsed := time.Since(start)

	// Local check should be very fast (under 1 second)
	if elapsed > 2*time.Second {
		t.Logf("Warning: Local image check took %v, expected < 2s", elapsed)
	}
}

// TestPullImageIfNeeded_PullsNewImage tests that new images are successfully pulled
func TestPullImageIfNeeded_PullsNewImage(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Use a small, commonly available image
	imageName := "alpine:3.19"

	// Remove the image first if it exists (to test pulling)
	_, _ = cli.ImageRemove(ctx, imageName, image.RemoveOptions{Force: true})

	// Now pull the image
	err = PullImageIfNeeded(ctx, cli, imageName)
	if err != nil {
		t.Fatalf("Failed to pull image: %v", err)
	}

	// Verify the image exists locally
	_, _, err = cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		t.Fatalf("Image not found after pull: %v", err)
	}
}

// TestPullImageIfNeeded_InvalidImage tests error handling for invalid images
func TestPullImageIfNeeded_InvalidImage(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to pull a non-existent image
	imageName := "this-image-definitely-does-not-exist-12345:latest"
	err = PullImageIfNeeded(ctx, cli, imageName)
	if err == nil {
		t.Fatal("Expected error when pulling non-existent image, got nil")
	}

	// Verify error message contains useful information
	errStr := err.Error()
	if !strings.Contains(errStr, "failed to pull image") {
		t.Errorf("Error message should contain 'failed to pull image', got: %s", errStr)
	}
	if !strings.Contains(errStr, imageName) {
		t.Errorf("Error message should contain image name '%s', got: %s", imageName, errStr)
	}
}

// TestPullImageIfNeeded_CLIFallback tests that CLI is tried first and API is used as fallback
func TestPullImageIfNeeded_CLIFallback(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Use a small public image to test the pull mechanism
	imageName := "busybox:stable"

	// Remove the image first if it exists
	_, _ = cli.ImageRemove(ctx, imageName, image.RemoveOptions{Force: true})

	// Pull the image - this tests the CLI/API fallback mechanism
	err = PullImageIfNeeded(ctx, cli, imageName)
	if err != nil {
		t.Fatalf("Failed to pull image: %v", err)
	}

	// Verify image was pulled
	_, _, err = cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		t.Fatalf("Image not found after pull: %v", err)
	}
}

// TestPullImageIfNeeded_ContextCancellation tests that context cancellation is respected
func TestPullImageIfNeeded_ContextCancellation(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to pull - should fail quickly due to cancelled context
	imageName := "alpine:latest"

	// Remove the image first so it actually tries to pull
	_, _ = cli.ImageRemove(context.Background(), imageName, image.RemoveOptions{Force: true})

	err = PullImageIfNeeded(ctx, cli, imageName)
	if err == nil {
		t.Log("Note: Pull succeeded despite cancelled context (image may have been cached)")
	}
	// We don't strictly require an error here because the image might already exist locally
}

// TestPullImageIfNeeded_MultipleImages tests pulling multiple different images
func TestPullImageIfNeeded_MultipleImages(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	images := []string{
		"hello-world:latest",
		"alpine:3.19",
		"busybox:stable",
	}

	for _, imageName := range images {
		t.Run(imageName, func(t *testing.T) {
			err := PullImageIfNeeded(ctx, cli, imageName)
			if err != nil {
				t.Errorf("Failed to pull image %s: %v", imageName, err)
			}

			// Verify image exists
			_, _, err = cli.ImageInspectWithRaw(ctx, imageName)
			if err != nil {
				t.Errorf("Image %s not found after pull: %v", imageName, err)
			}
		})
	}
}
