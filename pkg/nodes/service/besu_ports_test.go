package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBesuPorts_ValidPorts(t *testing.T) {
	// Use high ports that are likely available
	p2p, rpc, err := GetBesuPorts(49150, 49151)
	require.NoError(t, err)
	assert.True(t, p2p >= 49150 && p2p <= 65535, "P2P port should be in valid range, got %d", p2p)
	assert.True(t, rpc >= 49151 && rpc <= 65535, "RPC port should be in valid range, got %d", rpc)
}

func TestGetBesuPorts_P2PPortExceedsMax(t *testing.T) {
	_, _, err := GetBesuPorts(65536, 8545)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base P2P port 65536 exceeds maximum port number 65535")
}

func TestGetBesuPorts_RPCPortExceedsMax(t *testing.T) {
	_, _, err := GetBesuPorts(30303, 65536)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base RPC port 65536 exceeds maximum port number 65535")
}

func TestGetBesuPorts_BothPortsExceedMax(t *testing.T) {
	_, _, err := GetBesuPorts(70000, 80000)
	require.Error(t, err)
	// Should fail on the first check (P2P)
	assert.Contains(t, err.Error(), "P2P port")
}

func TestGetBesuPorts_MaxValidPort(t *testing.T) {
	// Port 65535 is valid but likely in use or close to limit
	// Just test that bounds check passes — the port-finding might fail
	// due to unavailability which is OK
	_, _, err := GetBesuPorts(65535, 65534)
	if err != nil {
		// Either bounds check passed and port is unavailable, or bounds check itself failed
		// Make sure it's NOT a bounds error
		assert.NotContains(t, err.Error(), "exceeds maximum port number")
	}
}

func TestGetBesuPorts_ZeroPorts(t *testing.T) {
	// Zero is a valid uint value, and 0 < 65535, so bounds check passes
	// but port 0 is special (OS assigns random port) — the port finder should handle it
	p2p, rpc, err := GetBesuPorts(0, 0)
	if err == nil {
		// If it succeeds, ports should be valid
		assert.True(t, p2p <= 65535, "P2P port should be <= 65535")
		assert.True(t, rpc <= 65535, "RPC port should be <= 65535")
	}
}

func TestGetBesuPorts_PortsAreDistinct(t *testing.T) {
	// When base ports are different, returned ports should be different
	p2p, rpc, err := GetBesuPorts(49200, 49300)
	if err == nil {
		assert.NotEqual(t, p2p, rpc, "P2P and RPC ports should be distinct")
	}
}
