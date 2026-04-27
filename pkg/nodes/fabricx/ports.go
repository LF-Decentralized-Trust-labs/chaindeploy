package fabricx

import (
	"fmt"
	"net"
	"time"
)

// findFreePort scans tcp ports starting at start (inclusive) returning the
// first one that can be bound on both 0.0.0.0 and 127.0.0.1. Both bind sites
// matter: docker publishes on 0.0.0.0 but localhost-only services on the
// host can still hold 127.0.0.1, and a half-occupied port would silently
// shadow the published metrics endpoint. Returns an error after scanning
// `windowSize` ports.
//
// We keep this self-contained instead of importing pkg/nodes/service to
// avoid an import cycle (service already depends on fabricx).
func findFreePort(start, windowSize int) (int, error) {
	if windowSize <= 0 {
		windowSize = 200
	}
	for p := start; p < start+windowSize; p++ {
		if isPortFree(p) {
			return p, nil
		}
		// Mirror pkg/nodes/service/ports.go's small sleep to avoid
		// hammering the kernel when scanning large ranges.
		time.Sleep(5 * time.Millisecond)
	}
	return 0, fmt.Errorf("no free tcp port found in [%d, %d)", start, start+windowSize)
}

// findFreePortsExcluding finds n free ports starting at start, skipping any
// port already in `exclude`. Used to allocate per-role monitoring ports
// without colliding with the GRPC ports just allocated for the same group.
func findFreePortsExcluding(start, n int, exclude map[int]struct{}) ([]int, error) {
	out := make([]int, 0, n)
	p := start
	for len(out) < n {
		nextStart := p
		port, err := findFreePort(nextStart, 1000)
		if err != nil {
			return nil, err
		}
		if _, used := exclude[port]; used {
			p = port + 1
			continue
		}
		out = append(out, port)
		p = port + 1
	}
	return out, nil
}

func isPortFree(port int) bool {
	for _, addr := range []string{"0.0.0.0", "127.0.0.1"} {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			return false
		}
		_ = ln.Close()
	}
	return true
}
