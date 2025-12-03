package orderer

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
		{"valid port 7050", "7050", false},
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
		{"valid localhost:7050", "localhost:7050", "listenAddress", false},
		{"valid 0.0.0.0:7050", "0.0.0.0:7050", "listenAddress", false},
		{"valid 127.0.0.1:7050", "127.0.0.1:7050", "listenAddress", false},
		{"valid with domain", "orderer0.org1.example.com:7050", "externalEndpoint", false},
		{"invalid missing port", "localhost", "listenAddress", true},
		{"invalid empty", "", "listenAddress", true},
		{"invalid port out of range", "localhost:70000", "listenAddress", true},
		{"invalid missing host", ":7050", "listenAddress", true},
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
		{"valid domain", "orderer0.org1.example.com", false},
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
		{"valid simple", "orderer0", false},
		{"valid with hyphen", "orderer-0", false},
		{"valid with underscore", "orderer_0", false},
		{"valid with space", "orderer 0", false},
		{"valid mixed", "Orderer-0_test", false},
		{"invalid empty", "", true},
		{"invalid special chars", "orderer@0", true},
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

func TestStartOrdererOpts_Validate(t *testing.T) {
	validOpts := StartOrdererOpts{
		ID:                      "orderer0",
		ListenAddress:           "0.0.0.0:7050",
		OperationsListenAddress: "0.0.0.0:9443",
		AdminListenAddress:      "0.0.0.0:7053",
		Version:                 "2.5.0",
	}

	tests := []struct {
		name    string
		opts    StartOrdererOpts
		wantErr bool
	}{
		{
			name:    "valid configuration",
			opts:    validOpts,
			wantErr: false,
		},
		{
			name: "invalid - empty ID",
			opts: StartOrdererOpts{
				ID:                      "",
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:7053",
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "invalid - bad listen address",
			opts: StartOrdererOpts{
				ID:                      "orderer0",
				ListenAddress:           "invalid",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:7053",
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "invalid - port conflict",
			opts: StartOrdererOpts{
				ID:                      "orderer0",
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:7050", // Same port
				AdminListenAddress:      "0.0.0.0:7053",
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "valid with external endpoint",
			opts: StartOrdererOpts{
				ID:                      "orderer0",
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:7053",
				ExternalEndpoint:        "orderer0.org1.example.com:7050",
				Version:                 "2.5.0",
			},
			wantErr: false,
		},
		{
			name: "valid with domain names",
			opts: StartOrdererOpts{
				ID:                      "orderer0",
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:7053",
				DomainNames:             []string{"orderer0.org1.example.com", "localhost"},
				Version:                 "2.5.0",
			},
			wantErr: false,
		},
		{
			name: "invalid domain name",
			opts: StartOrdererOpts{
				ID:                      "orderer0",
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:7053",
				DomainNames:             []string{"invalid@domain"},
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "valid with address overrides",
			opts: StartOrdererOpts{
				ID:                      "orderer0",
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:7053",
				Version:                 "2.5.0",
				AddressOverrides: []types.AddressOverride{
					{From: "orderer0:7050", To: "orderer0.org1.example.com:7050"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - bad admin address",
			opts: StartOrdererOpts{
				ID:                      "orderer0",
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "invalid-address",
				Version:                 "2.5.0",
			},
			wantErr: true,
		},
		{
			name: "invalid - three-way port conflict",
			opts: StartOrdererOpts{
				ID:                      "orderer0",
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:7050",
				AdminListenAddress:      "0.0.0.0:7050",
				Version:                 "2.5.0",
			},
			wantErr: true,
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
		opts    StartOrdererOpts
		wantErr bool
	}{
		{
			name: "no conflicts",
			opts: StartOrdererOpts{
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:7053",
			},
			wantErr: false,
		},
		{
			name: "listen and operations conflict",
			opts: StartOrdererOpts{
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:7050",
				AdminListenAddress:      "0.0.0.0:7053",
			},
			wantErr: true,
		},
		{
			name: "listen and admin conflict",
			opts: StartOrdererOpts{
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:7050",
			},
			wantErr: true,
		},
		{
			name: "operations and admin conflict",
			opts: StartOrdererOpts{
				ListenAddress:           "0.0.0.0:7050",
				OperationsListenAddress: "0.0.0.0:9443",
				AdminListenAddress:      "0.0.0.0:9443",
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
