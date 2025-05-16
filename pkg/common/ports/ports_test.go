package ports

import (
	"net"
	"strconv"
	"testing"
)

func TestGetFreePort(t *testing.T) {
	// Test getting a free port for a known node type
	allocation, err := GetFreePort("fabric-peer")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	if allocation.NodeType != "fabric-peer" {
		t.Errorf("Expected node type 'fabric-peer', got '%s'", allocation.NodeType)
	}

	if allocation.Port < 7051 || allocation.Port > 7151 {
		t.Errorf("Port %d outside expected range 7051-7151", allocation.Port)
	}

	// Test that the port is actually free
	addr := net.JoinHostPort("", strconv.Itoa(allocation.Port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Errorf("Port %d is not actually free: %v", allocation.Port, err)
	}
	listener.Close()

	// Test getting a port for unknown node type
	_, err = GetFreePort("unknown-type")
	if err == nil {
		t.Error("Expected error for unknown node type, got nil")
	}
}

func TestReleasePort(t *testing.T) {
	// Get a port first
	allocation, err := GetFreePort("fabric-peer")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	// Test releasing the port
	err = ReleasePort(allocation.Port)
	if err != nil {
		t.Errorf("Failed to release port: %v", err)
	}

	// Test releasing a non-allocated port
	err = ReleasePort(9999)
	if err == nil {
		t.Error("Expected error when releasing non-allocated port, got nil")
	}
}

func TestGetFreePorts(t *testing.T) {
	// Test getting multiple ports
	allocations, err := GetFreePorts("fabric-peer", 3)
	if err != nil {
		t.Fatalf("Failed to get free ports: %v", err)
	}

	if len(allocations) != 3 {
		t.Errorf("Expected 3 ports, got %d", len(allocations))
	}

	// Verify all ports are unique
	ports := make(map[int]bool)
	for _, alloc := range allocations {
		if ports[alloc.Port] {
			t.Errorf("Duplicate port %d allocated", alloc.Port)
		}
		ports[alloc.Port] = true
	}

	// Clean up
	for _, alloc := range allocations {
		ReleasePort(alloc.Port)
	}
}

func TestIsPortAvailable(t *testing.T) {
	// Find a free port in the range
	portRange, err := GetPortRange("fabric-peer")
	if err != nil {
		t.Fatalf("Failed to get port range: %v", err)
	}

	var freePort int
	for p := portRange.Start; p <= portRange.End; p++ {
		if IsPortAvailable(p) {
			freePort = p
			break
		}
	}
	if freePort == 0 {
		t.Fatal("No free port found in range")
	}

	// Test with a free port
	if !IsPortAvailable(freePort) {
		t.Errorf("Port %d should be available", freePort)
	}

	// Allocate a port
	allocation, err := GetFreePort("fabric-peer")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	// Test with an allocated port
	if IsPortAvailable(allocation.Port) {
		t.Errorf("Port %d should not be available", allocation.Port)
	}

	// Clean up
	ReleasePort(allocation.Port)
}

func TestGetPortRange(t *testing.T) {
	// Test getting range for known node type
	portRange, err := GetPortRange("fabric-peer")
	if err != nil {
		t.Fatalf("Failed to get port range: %v", err)
	}

	if portRange.Start != 7051 || portRange.End != 7151 {
		t.Errorf("Expected range 7051-7151, got %d-%d", portRange.Start, portRange.End)
	}

	// Test getting range for unknown node type
	_, err = GetPortRange("unknown-type")
	if err == nil {
		t.Error("Expected error for unknown node type, got nil")
	}
}

func TestAddPortRange(t *testing.T) {
	// Test adding valid port range
	err := AddPortRange("test-type", 8000, 8100)
	if err != nil {
		t.Errorf("Failed to add port range: %v", err)
	}

	// Verify the range was added
	portRange, err := GetPortRange("test-type")
	if err != nil {
		t.Fatalf("Failed to get port range: %v", err)
	}

	if portRange.Start != 8000 || portRange.End != 8100 {
		t.Errorf("Expected range 8000-8100, got %d-%d", portRange.Start, portRange.End)
	}

	// Test adding invalid port range
	err = AddPortRange("invalid-range", 9000, 8000)
	if err == nil {
		t.Error("Expected error for invalid port range, got nil")
	}
}

func TestGetAllocatedPorts(t *testing.T) {
	// Get some ports first
	allocations, err := GetFreePorts("fabric-peer", 2)
	if err != nil {
		t.Fatalf("Failed to get free ports: %v", err)
	}

	// Get all allocated ports
	allocatedPorts := GetAllocatedPorts()

	// Verify our ports are in the map
	for _, alloc := range allocations {
		if nodeType, exists := allocatedPorts[alloc.Port]; !exists || nodeType != "fabric-peer" {
			t.Errorf("Port %d not found in allocated ports or wrong node type", alloc.Port)
		}
	}

	// Clean up
	for _, alloc := range allocations {
		ReleasePort(alloc.Port)
	}
}
