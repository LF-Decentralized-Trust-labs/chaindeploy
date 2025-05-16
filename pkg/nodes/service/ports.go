package service

import (
	"fmt"
	"net"
	"time"
)

// isPortAvailable checks if a port is available by attempting to listen on it
func isPortAvailable(port int) bool {
	addrs := []string{
		"0.0.0.0",
		"127.0.0.1",
	}
	for _, addr := range addrs {
		fullAddr := fmt.Sprintf("%s:%d", addr, port)
		ln, err := net.Listen("tcp", fullAddr)
		if err != nil {
			return false
		}
		ln.Close()
	}
	return true
}

// findConsecutivePorts finds n consecutive available ports starting from startPort
func findConsecutivePorts(startPort, count, maxPort int) ([]int, error) {
	if maxPort == 0 {
		maxPort = startPort + 1000
	}

	currentPort := startPort
	for currentPort <= maxPort-count+1 {
		// Generate candidate ports
		candidatePorts := make([]int, count)
		for i := 0; i < count; i++ {
			candidatePorts[i] = currentPort + i
		}

		// Check all ports
		allAvailable := true
		firstUnavailable := 0
		for i, port := range candidatePorts {
			if !isPortAvailable(port) {
				allAvailable = false
				firstUnavailable = i
				break
			}
			// Add a small delay to prevent overwhelming the system
			time.Sleep(10 * time.Millisecond)
		}

		if allAvailable {
			return candidatePorts, nil
		}

		// Skip to next port after the first unavailable one
		currentPort = candidatePorts[firstUnavailable] + 1
	}

	return nil, fmt.Errorf("no %d consecutive ports available starting from %d", count, startPort)
}

// Update GetPeerPorts to use sequential port checking
func GetPeerPorts(basePort int) (listen, chaincode, events, operations int, err error) {
	maxAttempts := 100 // Maximum number of attempts to find available ports
	currentBase := basePort

	for attempt := 0; attempt < maxAttempts; attempt++ {
		listen = currentBase
		chaincode = currentBase + 1
		events = currentBase + 2
		operations = currentBase + 3

		// Check if all ports are available
		allAvailable := true
		ports := []int{listen, chaincode, events, operations}
		for _, port := range ports {
			if !isPortAvailable(port) {
				allAvailable = false
				break
			}
		}

		if allAvailable {
			return listen, chaincode, events, operations, nil
		}

		// If not all ports are available, try the next block of ports
		currentBase += 4
	}

	return 0, 0, 0, 0, fmt.Errorf("no available port block found after %d attempts starting from %d", maxAttempts, basePort)
}

// Update GetOrdererPorts to use sequential port checking
func GetOrdererPorts(basePort int) (listen, admin, operations int, err error) {
	maxAttempts := 100 // Maximum number of attempts to find available ports
	currentBase := basePort

	for attempt := 0; attempt < maxAttempts; attempt++ {
		listen = currentBase
		admin = currentBase + 1
		operations = currentBase + 2

		// Check if all ports are available
		allAvailable := true
		ports := []int{listen, admin, operations}
		for _, port := range ports {
			if !isPortAvailable(port) {
				allAvailable = false
				break
			}
		}

		if allAvailable {
			return listen, admin, operations, nil
		}

		// If not all ports are available, try the next block of ports
		currentBase += 3
	}

	return 0, 0, 0, fmt.Errorf("no available port block found after %d attempts starting from %d", maxAttempts, basePort)
}
