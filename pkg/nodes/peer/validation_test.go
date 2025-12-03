package peer

import (
	"strings"
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{"valid port 7051", "7051", false},
		{"valid port 443", "443", false},
		{"valid port 1", "1", false},
		{"valid port 65535", "65535", false},
		{"invalid port 0", "0", true},
		{"invalid port 65536", "65536", true},
		{"invalid port negative", "-1", true},
		{"invalid port empty", "", true},
		{"invalid port non-numeric", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)
			if tt.wantErr && err == nil {
				t.Errorf("validatePort(%s) expected error, got nil", tt.port)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validatePort(%s) unexpected error: %v", tt.port, err)
			}
		})
	}
}

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		fieldName string
		wantErr   bool
	}{
		{"valid localhost:7051", "localhost:7051", "listenAddress", false},
		{"valid 0.0.0.0:7051", "0.0.0.0:7051", "listenAddress", false},
		{"valid 127.0.0.1:7051", "127.0.0.1:7051", "listenAddress", false},
		{"valid with domain", "peer0.org1.example.com:7051", "externalEndpoint", false},
		{"invalid missing port", "localhost", "listenAddress", true},
		{"invalid empty", "", "listenAddress", true},
		{"invalid port out of range", "localhost:70000", "listenAddress", true},
		{"invalid missing host", ":7051", "listenAddress", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAddress(tt.address, tt.fieldName)
			if tt.wantErr && err == nil {
				t.Errorf("validateAddress(%s, %s) expected error, got nil", tt.address, tt.fieldName)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateAddress(%s, %s) unexpected error: %v", tt.address, tt.fieldName, err)
			}
		})
	}
}

func TestValidateDomainName(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{"valid domain", "peer0.org1.example.com", false},
		{"valid short domain", "localhost", false},
		{"valid IP", "192.168.1.100", false},
		{"valid IPv6", "::1", false},
		{"invalid empty", "", true},
		{"invalid starts with hyphen", "-invalid.com", true},
		{"invalid ends with hyphen", "invalid-.com", true},
		{"invalid special chars", "invalid@domain.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDomainName(tt.domain)
			if tt.wantErr && err == nil {
				t.Errorf("validateDomainName(%s) expected error, got nil", tt.domain)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateDomainName(%s) unexpected error: %v", tt.domain, err)
			}
		})
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid simple", "peer0", false},
		{"valid with hyphen", "peer-0", false},
		{"valid with underscore", "peer_0", false},
		{"valid with space", "peer 0", false},
		{"valid mixed", "Peer-0_test", false},
		{"invalid empty", "", true},
		{"invalid special chars", "peer@0", true},
		{"invalid too long", strings.Repeat("a", 256), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateID(tt.id)
			if tt.wantErr && err == nil {
				t.Errorf("validateID(%s) expected error, got nil", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateID(%s) unexpected error: %v", tt.id, err)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"valid v2.5.0", "v2.5.0", false},
		{"valid 2.5.0", "2.5.0", false},
		{"valid 2.5", "2.5", false},
		{"valid with suffix", "2.5.0-alpha", false},
		{"invalid empty", "", true},
		{"invalid format", "version2.5", true},
		{"invalid random", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVersion(tt.version)
			if tt.wantErr && err == nil {
				t.Errorf("validateVersion(%s) expected error, got nil", tt.version)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateVersion(%s) unexpected error: %v", tt.version, err)
			}
		})
	}
}

func TestStartPeerOpts_Validate(t *testing.T) {
	validOpts := StartPeerOpts{
		ID:                      "peer0",
		ListenAddress:           "0.0.0.0:7051",
		ChaincodeAddress:        "0.0.0.0:7052",
		EventsAddress:           "0.0.0.0:7053",
		OperationsListenAddress: "0.0.0.0:9443",
		Version:                 "2.5.0",
	}

	tests := []struct {
		name    string
		opts    StartPeerOpts
		wantErr bool
	}{
		{
			name:    "valid configuration",
			opts:    validOpts,
			wantErr: false,
		},
		{
			name: "invalid - empty ID",
			opts: StartPeerOpts{
				ID:                      "",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "invalid - bad listen address",
			opts: StartPeerOpts{
				ID:                      "peer0",
				ListenAddress:           "invalid",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "invalid - port conflict",
			opts: StartPeerOpts{
				ID:                      "peer0",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7051", // Same port
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "valid with external endpoint",
			opts: StartPeerOpts{
				ID:                      "peer0",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				ExternalEndpoint:        "peer0.org1.example.com:7051",
				Version:                 "2.5.0",
			},
			wantErr: false,
		},
		{
			name: "valid with domain names",
			opts: StartPeerOpts{
				ID:                      "peer0",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				DomainNames:             []string{"peer0.org1.example.com", "localhost"},
				Version:                 "2.5.0",
			},
			wantErr: false,
		},
		{
			name: "invalid domain name",
			opts: StartPeerOpts{
				ID:                      "peer0",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				DomainNames:             []string{"invalid@domain"},
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "valid with address overrides",
			opts: StartPeerOpts{
				ID:                      "peer0",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				Version:                 "2.5.0",
				AddressOverrides: []types.AddressOverride{
					{From: "peer0:7051", To: "peer0.org1.example.com:7051"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestCheckPortConflicts(t *testing.T) {
	tests := []struct {
		name    string
		opts    StartPeerOpts
		wantErr bool
	}{
		{
			name: "no conflicts",
			opts: StartPeerOpts{
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
			},
			wantErr: false,
		},
		{
			name: "listen and chaincode conflict",
			opts: StartPeerOpts{
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7051",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
			},
			wantErr: true,
		},
		{
			name: "events and operations conflict",
			opts: StartPeerOpts{
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:9443",
				OperationsListenAddress: "0.0.0.0:9443",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.checkPortConflicts()
			if tt.wantErr && err == nil {
				t.Error("Expected port conflict error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected port conflict error: %v", err)
			}
		})
	}
}
