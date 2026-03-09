package service

import (
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// --- extractPort tests ---

func TestExtractPort_HostPort(t *testing.T) {
	cases := []struct {
		address  string
		expected string
	}{
		{"0.0.0.0:7051", "7051"},
		{"127.0.0.1:8545", "8545"},
		{"localhost:30303", "30303"},
		{"[::1]:9090", "9090"},
	}
	for _, tc := range cases {
		got := extractPort(tc.address)
		if got != tc.expected {
			t.Errorf("extractPort(%q) = %q, want %q", tc.address, got, tc.expected)
		}
	}
}

func TestExtractPort_Empty(t *testing.T) {
	if got := extractPort(""); got != "" {
		t.Errorf("extractPort(\"\") = %q, want empty", got)
	}
}

func TestExtractPort_InvalidFormat(t *testing.T) {
	// No port separator
	if got := extractPort("noporthere"); got != "" {
		t.Errorf("extractPort(\"noporthere\") = %q, want empty", got)
	}
}

// --- extractPeerPorts tests ---

func TestExtractPeerPorts_AllSet(t *testing.T) {
	config := &types.FabricPeerConfig{
		ListenAddress:           "0.0.0.0:7051",
		ChaincodeAddress:        "0.0.0.0:7052",
		EventsAddress:           "0.0.0.0:7053",
		OperationsListenAddress: "0.0.0.0:9443",
	}
	ports := extractPeerPorts(1, "peer0", config)
	if len(ports) != 4 {
		t.Fatalf("expected 4 ports, got %d", len(ports))
	}
	// Verify purposes
	purposes := make(map[string]bool)
	for _, p := range ports {
		purposes[p.Purpose] = true
		if p.NodeID != 1 {
			t.Errorf("expected NodeID 1, got %d", p.NodeID)
		}
		if p.NodeName != "peer0" {
			t.Errorf("expected NodeName 'peer0', got %q", p.NodeName)
		}
		if p.NodeType != types.NodeTypeFabricPeer {
			t.Errorf("expected NodeType %q, got %q", types.NodeTypeFabricPeer, p.NodeType)
		}
	}
	for _, expected := range []string{"listen", "chaincode", "events", "operations"} {
		if !purposes[expected] {
			t.Errorf("missing purpose %q", expected)
		}
	}
}

func TestExtractPeerPorts_PartiallySet(t *testing.T) {
	config := &types.FabricPeerConfig{
		ListenAddress: "0.0.0.0:7051",
		// Others empty
	}
	ports := extractPeerPorts(1, "peer0", config)
	if len(ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(ports))
	}
	if ports[0].Port != "7051" {
		t.Errorf("expected port 7051, got %s", ports[0].Port)
	}
}

func TestExtractPeerPorts_NoneSet(t *testing.T) {
	config := &types.FabricPeerConfig{}
	ports := extractPeerPorts(1, "peer0", config)
	if len(ports) != 0 {
		t.Fatalf("expected 0 ports, got %d", len(ports))
	}
}

// --- extractOrdererPorts tests ---

func TestExtractOrdererPorts_AllSet(t *testing.T) {
	config := &types.FabricOrdererConfig{
		ListenAddress:           "0.0.0.0:7050",
		AdminAddress:            "0.0.0.0:7053",
		OperationsListenAddress: "0.0.0.0:8443",
	}
	ports := extractOrdererPorts(2, "orderer0", config)
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(ports))
	}
	purposes := make(map[string]bool)
	for _, p := range ports {
		purposes[p.Purpose] = true
		if p.NodeType != types.NodeTypeFabricOrderer {
			t.Errorf("expected NodeType %q, got %q", types.NodeTypeFabricOrderer, p.NodeType)
		}
	}
	for _, expected := range []string{"listen", "admin", "operations"} {
		if !purposes[expected] {
			t.Errorf("missing purpose %q", expected)
		}
	}
}

func TestExtractOrdererPorts_NoneSet(t *testing.T) {
	config := &types.FabricOrdererConfig{}
	ports := extractOrdererPorts(2, "orderer0", config)
	if len(ports) != 0 {
		t.Fatalf("expected 0 ports, got %d", len(ports))
	}
}

// --- extractBesuPorts tests ---

func TestExtractBesuPorts_NoMetrics(t *testing.T) {
	config := &types.BesuNodeConfig{
		P2PPort:        30303,
		RPCPort:        8545,
		MetricsEnabled: false,
	}
	ports := extractBesuPorts(3, "besu0", config)
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	purposes := make(map[string]string)
	for _, p := range ports {
		purposes[p.Purpose] = p.Port
		if p.NodeType != types.NodeTypeBesuFullnode {
			t.Errorf("expected NodeType %q, got %q", types.NodeTypeBesuFullnode, p.NodeType)
		}
	}
	if purposes["p2p"] != "30303" {
		t.Errorf("expected p2p port 30303, got %s", purposes["p2p"])
	}
	if purposes["rpc"] != "8545" {
		t.Errorf("expected rpc port 8545, got %s", purposes["rpc"])
	}
}

func TestExtractBesuPorts_WithMetrics(t *testing.T) {
	config := &types.BesuNodeConfig{
		P2PPort:        30303,
		RPCPort:        8545,
		MetricsEnabled: true,
		MetricsPort:    9545,
	}
	ports := extractBesuPorts(3, "besu0", config)
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(ports))
	}
	purposes := make(map[string]string)
	for _, p := range ports {
		purposes[p.Purpose] = p.Port
	}
	if purposes["metrics"] != "9545" {
		t.Errorf("expected metrics port 9545, got %s", purposes["metrics"])
	}
}

// --- PortUsage struct tests ---

func TestPortUsage_Fields(t *testing.T) {
	pu := PortUsage{
		NodeID:   42,
		NodeName: "test-node",
		NodeType: types.NodeTypeBesuFullnode,
		Port:     "8545",
		Purpose:  "rpc",
	}
	if pu.NodeID != 42 {
		t.Errorf("expected NodeID 42, got %d", pu.NodeID)
	}
	if pu.NodeName != "test-node" {
		t.Errorf("expected NodeName 'test-node', got %q", pu.NodeName)
	}
	if pu.NodeType != types.NodeTypeBesuFullnode {
		t.Errorf("expected NodeType BESU_FULLNODE, got %q", pu.NodeType)
	}
	if pu.Port != "8545" {
		t.Errorf("expected Port '8545', got %q", pu.Port)
	}
	if pu.Purpose != "rpc" {
		t.Errorf("expected Purpose 'rpc', got %q", pu.Purpose)
	}
}
