package metrics

import (
	"fmt"
	"net"
	"sync"
)

var (
	// Prometheus port range starting from 9090
	PrometheusPortRange = struct {
		Start int
		End   int
	}{
		Start: 9090,
		End:   9190, // 100 ports range
	}

	// Mutex to protect port allocation
	prometheusPortMutex sync.Mutex
	// Map to track allocated Prometheus ports
	allocatedPrometheusPorts = make(map[int]bool)
)

// FindAvailablePrometheusPort finds an available port for Prometheus starting from 9090
func FindAvailablePrometheusPort() (int, error) {
	prometheusPortMutex.Lock()
	defer prometheusPortMutex.Unlock()

	// Try to find a free port in the range
	for port := PrometheusPortRange.Start; port <= PrometheusPortRange.End; port++ {
		if allocatedPrometheusPorts[port] {
			continue // Port is already allocated
		}

		// Check if port is actually free
		if isPortAvailable(port) {
			allocatedPrometheusPorts[port] = true
			return port, nil
		}
	}

	return 0, fmt.Errorf("no free ports available in range %d-%d for Prometheus",
		PrometheusPortRange.Start, PrometheusPortRange.End)
}

// CheckPrometheusPortAvailability checks if a specific port is available for Prometheus
func CheckPrometheusPortAvailability(port int) bool {
	prometheusPortMutex.Lock()
	defer prometheusPortMutex.Unlock()

	if allocatedPrometheusPorts[port] {
		return false
	}

	return isPortAvailable(port)
}

// ReleasePrometheusPort releases an allocated Prometheus port
func ReleasePrometheusPort(port int) error {
	prometheusPortMutex.Lock()
	defer prometheusPortMutex.Unlock()

	if !allocatedPrometheusPorts[port] {
		return fmt.Errorf("port %d is not allocated", port)
	}

	delete(allocatedPrometheusPorts, port)
	return nil
}

// GetAvailablePrometheusPorts returns a list of available ports in the Prometheus range
func GetAvailablePrometheusPorts(count int) ([]int, error) {
	prometheusPortMutex.Lock()
	defer prometheusPortMutex.Unlock()

	var availablePorts []int
	for port := PrometheusPortRange.Start; port <= PrometheusPortRange.End && len(availablePorts) < count; port++ {
		if !allocatedPrometheusPorts[port] && isPortAvailable(port) {
			availablePorts = append(availablePorts, port)
		}
	}

	if len(availablePorts) < count {
		return nil, fmt.Errorf("only %d ports available, requested %d", len(availablePorts), count)
	}

	return availablePorts, nil
}

// isPortAvailable checks if a port is available by attempting to listen on it
func isPortAvailable(port int) bool {
	addrs := []string{
		"0.0.0.0",
		"127.0.0.1",
	}
	for _, addr := range addrs {
		fullAddr := fmt.Sprintf("%s:%d", addr, port)
		listener, err := net.Listen("tcp", fullAddr)
		if err != nil {
			return false
		}
		listener.Close()
	}
	return true
}

// GetAllocatedPrometheusPorts returns a list of all currently allocated Prometheus ports
func GetAllocatedPrometheusPorts() []int {
	prometheusPortMutex.Lock()
	defer prometheusPortMutex.Unlock()

	ports := make([]int, 0, len(allocatedPrometheusPorts))
	for port := range allocatedPrometheusPorts {
		ports = append(ports, port)
	}
	return ports
}
