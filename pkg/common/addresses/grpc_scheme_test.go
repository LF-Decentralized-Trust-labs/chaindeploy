package addresses

import (
	"strings"
	"testing"
)

// stripGrpcScheme replicates the inline URL stripping pattern used throughout
// the codebase (deployer.go, org.go, orderer.go, peer.go) to strip grpcs://
// and grpc:// prefixes before passing addresses to gRPC dial functions that
// expect host:port only.
func stripGrpcScheme(url string) string {
	addr := strings.TrimPrefix(url, "grpcs://")
	addr = strings.TrimPrefix(addr, "grpc://")
	return addr
}

func TestStripGrpcScheme(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "grpcs scheme is stripped",
			input:    "grpcs://host:7050",
			expected: "host:7050",
		},
		{
			name:     "grpc scheme is stripped",
			input:    "grpc://host:7050",
			expected: "host:7050",
		},
		{
			name:     "no scheme remains unchanged",
			input:    "host:7050",
			expected: "host:7050",
		},
		{
			name:     "grpcs scheme only yields empty string",
			input:    "grpcs://",
			expected: "",
		},
		{
			name:     "grpc scheme only yields empty string",
			input:    "grpc://",
			expected: "",
		},
		{
			name:     "empty string remains empty",
			input:    "",
			expected: "",
		},
		{
			name:     "grpcs with IP address",
			input:    "grpcs://192.168.1.1:7050",
			expected: "192.168.1.1:7050",
		},
		{
			name:     "grpc with IP address",
			input:    "grpc://192.168.1.1:7050",
			expected: "192.168.1.1:7050",
		},
		{
			name:     "grpcs with hostname and no port",
			input:    "grpcs://orderer.example.com",
			expected: "orderer.example.com",
		},
		{
			name:     "https scheme is not stripped",
			input:    "https://host:7050",
			expected: "https://host:7050",
		},
		{
			name:     "http scheme is not stripped",
			input:    "http://host:7050",
			expected: "http://host:7050",
		},
		{
			name:     "grpcs in hostname is not stripped",
			input:    "grpcs.example.com:7050",
			expected: "grpcs.example.com:7050",
		},
		{
			name:     "uppercase GRPCS is not stripped",
			input:    "GRPCS://host:7050",
			expected: "GRPCS://host:7050",
		},
		{
			name:     "grpcs with localhost",
			input:    "grpcs://localhost:7050",
			expected: "localhost:7050",
		},
		{
			name:     "grpc with localhost",
			input:    "grpc://localhost:7050",
			expected: "localhost:7050",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripGrpcScheme(tt.input)
			if result != tt.expected {
				t.Errorf("stripGrpcScheme(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
