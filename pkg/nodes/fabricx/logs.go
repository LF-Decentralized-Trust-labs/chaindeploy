package fabricx

import (
	"context"
	"fmt"
	"io"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

// TailContainerLogs streams stdout+stderr from a FabricX container via the
// Docker API, returning a channel of log lines. Callers receive the same
// shape as the Fabric peer/orderer TailLogs so the generic service layer
// can treat every node type uniformly.
//
// Docker multiplexes stdout/stderr with an 8-byte header per frame
// ([stream, 0, 0, 0, size_be_uint32]) when TTY is false, which is the case
// for all chaindeploy-managed containers; this function demuxes that.
func TailContainerLogs(ctx context.Context, log *logger.Logger, containerName string, tail int, follow bool) (<-chan string, error) {
	if containerName == "" {
		return nil, fmt.Errorf("fabricx: TailContainerLogs requires containerName")
	}

	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	if _, err := cli.ContainerInspect(ctx, containerName); err != nil {
		cli.Close()
		return nil, fmt.Errorf("container %q not found: %w", containerName, err)
	}

	logChan := make(chan string, 100)
	go func() {
		defer close(logChan)
		defer cli.Close()

		reader, err := cli.ContainerLogs(ctx, containerName, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     follow,
			Tail:       fmt.Sprintf("%d", tail),
		})
		if err != nil {
			log.Error("Failed to fetch docker logs", "container", containerName, "error", err)
			return
		}
		defer reader.Close()

		header := make([]byte, 8)
		for {
			if _, err := io.ReadFull(reader, header); err != nil {
				if err != io.EOF && err != io.ErrUnexpectedEOF {
					log.Error("Failed to read docker log header", "container", containerName, "error", err)
				}
				return
			}
			length := int(uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7]))
			if length == 0 {
				continue
			}
			payload := make([]byte, length)
			if _, err := io.ReadFull(reader, payload); err != nil {
				if err != io.EOF && err != io.ErrUnexpectedEOF {
					log.Error("Failed to read docker log payload", "container", containerName, "error", err)
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case logChan <- string(payload):
			}
		}
	}()
	return logChan, nil
}
