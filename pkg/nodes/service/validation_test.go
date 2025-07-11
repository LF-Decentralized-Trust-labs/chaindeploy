package service

import (
	"strings"
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

func TestValidateAddressFormat(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "valid address with port",
			address: "localhost:7051",
			wantErr: false,
		},
		{
			name:    "valid address with IP and port",
			address: "127.0.0.1:7051",
			wantErr: false,
		},
		{
			name:    "valid address with 0.0.0.0",
			address: "0.0.0.0:7051",
			wantErr: false,
		},
		{
			name:    "missing port",
			address: "localhost",
			wantErr: true,
		},
		{
			name:    "invalid port number",
			address: "localhost:99999",
			wantErr: true,
		},
		{
			name:    "port out of range",
			address: "localhost:0",
			wantErr: true,
		},
		{
			name:    "empty host",
			address: ":7051",
			wantErr: true,
		},
		{
			name:    "empty address",
			address: "",
			wantErr: true,
		},
	}

	svc := &NodeService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateAddressFormat(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAddressFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFabricPeerConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *types.FabricPeerConfig
		wantErr bool
	}{
		{
			name: "valid peer config",
			config: &types.FabricPeerConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-peer",
					Mode: "service",
				},
				Name:                    "peer0-org1",
				OrganizationID:          1,
				MSPID:                   "Org1MSP",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				ExternalEndpoint:        "peer0.org1.example.com:7051",
				DomainNames:             []string{"peer0.org1.example.com"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: &types.FabricPeerConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-peer",
					Mode: "service",
				},
				OrganizationID:          1,
				MSPID:                   "Org1MSP",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				ExternalEndpoint:        "peer0.org1.example.com:7051",
				DomainNames:             []string{"peer0.org1.example.com"},
			},
			wantErr: true,
		},
		{
			name: "invalid address format",
			config: &types.FabricPeerConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-peer",
					Mode: "service",
				},
				Name:                    "peer0-org1",
				OrganizationID:          1,
				MSPID:                   "Org1MSP",
				ListenAddress:           "invalid-address",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				ExternalEndpoint:        "peer0.org1.example.com:7051",
				DomainNames:             []string{"peer0.org1.example.com"},
			},
			wantErr: true,
		},
		{
			name: "port conflict",
			config: &types.FabricPeerConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-peer",
					Mode: "service",
				},
				Name:                    "peer0-org1",
				OrganizationID:          1,
				MSPID:                   "Org1MSP",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7051", // Same port as listen address
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				ExternalEndpoint:        "peer0.org1.example.com:7051",
				DomainNames:             []string{"peer0.org1.example.com"},
			},
			wantErr: true,
		},
		{
			name: "invalid deployment mode",
			config: &types.FabricPeerConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-peer",
					Mode: "invalid",
				},
				Name:                    "peer0-org1",
				OrganizationID:          1,
				MSPID:                   "Org1MSP",
				ListenAddress:           "0.0.0.0:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				ExternalEndpoint:        "peer0.org1.example.com:7051",
				DomainNames:             []string{"peer0.org1.example.com"},
			},
			wantErr: true,
		},
	}

	svc := &NodeService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateFabricPeerConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFabricPeerConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFabricOrdererConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *types.FabricOrdererConfig
		wantErr bool
	}{
		{
			name: "valid orderer config",
			config: &types.FabricOrdererConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-orderer",
					Mode: "service",
				},
				Name:                    "orderer.example.com",
				OrganizationID:          1,
				MSPID:                   "OrdererMSP",
				ListenAddress:           "0.0.0.0:7050",
				AdminAddress:            "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				ExternalEndpoint:        "orderer.example.com:7050",
				DomainNames:             []string{"orderer.example.com"},
			},
			wantErr: false,
		},
		{
			name: "port conflict",
			config: &types.FabricOrdererConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-orderer",
					Mode: "service",
				},
				Name:                    "orderer.example.com",
				OrganizationID:          1,
				MSPID:                   "OrdererMSP",
				ListenAddress:           "0.0.0.0:7050",
				AdminAddress:            "0.0.0.0:7050", // Same port as listen address
				OperationsListenAddress: "0.0.0.0:9443",
				ExternalEndpoint:        "orderer.example.com:7050",
				DomainNames:             []string{"orderer.example.com"},
			},
			wantErr: true,
		},
	}

	svc := &NodeService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateFabricOrdererConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFabricOrdererConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBesuNodeConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *types.BesuNodeConfig
		wantErr bool
	}{
		{
			name: "valid besu config",
			config: &types.BesuNodeConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "besu",
					Mode: "service",
				},
				NetworkID:  1337,
				KeyID:      1,
				P2PPort:    30303,
				RPCPort:    8545,
				P2PHost:    "0.0.0.0",
				RPCHost:    "0.0.0.0",
				ExternalIP: "127.0.0.1",
				InternalIP: "127.0.0.1",
			},
			wantErr: false,
		},
		{
			name: "port conflict",
			config: &types.BesuNodeConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "besu",
					Mode: "service",
				},
				NetworkID:  1337,
				KeyID:      1,
				P2PPort:    30303,
				RPCPort:    30303, // Same as P2P port
				P2PHost:    "0.0.0.0",
				RPCHost:    "0.0.0.0",
				ExternalIP: "127.0.0.1",
				InternalIP: "127.0.0.1",
			},
			wantErr: true,
		},
		{
			name: "invalid port range",
			config: &types.BesuNodeConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "besu",
					Mode: "service",
				},
				NetworkID:  1337,
				KeyID:      1,
				P2PPort:    80, // Below minimum
				RPCPort:    8545,
				P2PHost:    "0.0.0.0",
				RPCHost:    "0.0.0.0",
				ExternalIP: "127.0.0.1",
				InternalIP: "127.0.0.1",
			},
			wantErr: true,
		},
		{
			name: "invalid IP address",
			config: &types.BesuNodeConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "besu",
					Mode: "service",
				},
				NetworkID:  1337,
				KeyID:      1,
				P2PPort:    30303,
				RPCPort:    8545,
				P2PHost:    "0.0.0.0",
				RPCHost:    "0.0.0.0",
				ExternalIP: "invalid-ip",
				InternalIP: "127.0.0.1",
			},
			wantErr: true,
		},
	}

	svc := &NodeService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateBesuNodeConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBesuNodeConfig() error = %v, wantErr %v", err, tt.wantErr)
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
		{
			name:    "valid domain name",
			domain:  "example.com",
			wantErr: false,
		},
		{
			name:    "valid subdomain",
			domain:  "peer0.org1.example.com",
			wantErr: false,
		},
		{
			name:    "valid domain with numbers",
			domain:  "peer1-org2.example.com",
			wantErr: false,
		},
		{
			name:    "empty domain",
			domain:  "",
			wantErr: true,
		},
		{
			name:    "single part domain",
			domain:  "example",
			wantErr: false,
		},
		{
			name:    "domain starting with hyphen",
			domain:  "-example.com",
			wantErr: true,
		},
		{
			name:    "domain ending with hyphen",
			domain:  "example-.com",
			wantErr: true,
		},
		{
			name:    "domain with invalid characters",
			domain:  "example@.com",
			wantErr: true,
		},
		{
			name:    "domain with uppercase letters",
			domain:  "Example.com",
			wantErr: true,
		},
		{
			name:    "domain part too long",
			domain:  "a" + strings.Repeat("b", 63) + ".com",
			wantErr: true,
		},
	}

	svc := &NodeService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateDomainName(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDomainName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateExternalEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "valid domain endpoint",
			endpoint: "peer0.org1.example.com:7051",
			wantErr:  false,
		},
		{
			name:     "valid IP endpoint",
			endpoint: "192.168.1.100:7051",
			wantErr:  false,
		},
		{
			name:     "valid localhost endpoint",
			endpoint: "localhost:7051",
			wantErr:  false,
		},
		{
			name:     "missing port",
			endpoint: "peer0.org1.example.com",
			wantErr:  true,
		},
		{
			name:     "invalid port number",
			endpoint: "peer0.org1.example.com:99999",
			wantErr:  true,
		},
		{
			name:     "port out of range",
			endpoint: "peer0.org1.example.com:0",
			wantErr:  true,
		},
		{
			name:     "empty host",
			endpoint: ":7051",
			wantErr:  true,
		},
		{
			name:     "invalid domain in endpoint",
			endpoint: "invalid@domain.com:7051",
			wantErr:  true,
		},
	}

	svc := &NodeService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateExternalEndpoint(tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExternalEndpoint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
