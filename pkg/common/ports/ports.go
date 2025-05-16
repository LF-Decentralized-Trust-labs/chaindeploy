package ports

import (
	"fmt"
	"net"
	"sync"
)

var (
	// Default port ranges for different node types
	DefaultPortRanges = map[string]PortRange{
		"fabric-peer": {
			Start: 7051,
			End:   7151,
		},
		"fabric-orderer": {
			Start: 7050,
			End:   7150,
		},
		"fabric-ca": {
			Start: 7054,
			End:   7154,
		},
		"besu": {
			Start: 8545,
			End:   8645,
		},
		"besu-p2p": {
			Start: 30303,
			End:   30403,
		},
	}

	// Mutex to protect port allocation
	portMutex sync.Mutex
	// Map to track allocated ports
	allocatedPorts = make(map[int]string)
)

// PortRange represents a range of ports that can be allocated
type PortRange struct {
	Start int
	End   int
}

// PortAllocation represents an allocated port with its type
type PortAllocation struct {
	Port     int
	NodeType string
}

// GetFreePort finds a free port in the specified range
func GetFreePort(nodeType string) (*PortAllocation, error) {
	portMutex.Lock()
	defer portMutex.Unlock()

	// Get port range for node type
	portRange, exists := DefaultPortRanges[nodeType]
	if !exists {
		return nil, fmt.Errorf("unknown node type: %s", nodeType)
	}

	// Try to find a free port in the range
	for port := portRange.Start; port <= portRange.End; port++ {
		if _, allocated := allocatedPorts[port]; !allocated {
			// Check if port is actually free
			addr := fmt.Sprintf(":%d", port)
			listener, err := net.Listen("tcp", addr)
			if err == nil {
				listener.Close()
				allocatedPorts[port] = nodeType
				return &PortAllocation{
					Port:     port,
					NodeType: nodeType,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no free ports available in range %d-%d for node type %s",
		portRange.Start, portRange.End, nodeType)
}

// ReleasePort releases an allocated port
func ReleasePort(port int) error {
	portMutex.Lock()
	defer portMutex.Unlock()

	if _, exists := allocatedPorts[port]; !exists {
		return fmt.Errorf("port %d is not allocated", port)
	}

	delete(allocatedPorts, port)
	return nil
}

// GetFreePorts allocates multiple free ports for a node
func GetFreePorts(nodeType string, count int) ([]*PortAllocation, error) {
	allocations := make([]*PortAllocation, 0, count)

	for i := 0; i < count; i++ {
		allocation, err := GetFreePort(nodeType)
		if err != nil {
			// Release any previously allocated ports
			for _, alloc := range allocations {
				ReleasePort(alloc.Port)
			}
			return nil, fmt.Errorf("failed to allocate port %d: %w", i+1, err)
		}
		allocations = append(allocations, allocation)
	}

	return allocations, nil
}

// IsPortAvailable checks if a specific port is available
func IsPortAvailable(port int) bool {
	portMutex.Lock()
	defer portMutex.Unlock()

	if _, allocated := allocatedPorts[port]; allocated {
		return false
	}

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

// GetPortRange returns the port range for a specific node type
func GetPortRange(nodeType string) (*PortRange, error) {
	portRange, exists := DefaultPortRanges[nodeType]
	if !exists {
		return nil, fmt.Errorf("unknown node type: %s", nodeType)
	}
	return &portRange, nil
}

// AddPortRange adds a new port range for a node type
func AddPortRange(nodeType string, start, end int) error {
	portMutex.Lock()
	defer portMutex.Unlock()

	if start >= end {
		return fmt.Errorf("invalid port range: start (%d) must be less than end (%d)", start, end)
	}

	DefaultPortRanges[nodeType] = PortRange{
		Start: start,
		End:   end,
	}
	return nil
}

// GetAllocatedPorts returns a map of all currently allocated ports
func GetAllocatedPorts() map[int]string {
	portMutex.Lock()
	defer portMutex.Unlock()

	// Create a copy of the map to prevent external modification
	ports := make(map[int]string, len(allocatedPorts))
	for port, nodeType := range allocatedPorts {
		ports[port] = nodeType
	}
	return ports
}
