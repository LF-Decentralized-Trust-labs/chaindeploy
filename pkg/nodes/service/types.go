package service

import (
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// UpdateFabricPeerOpts represents the options for updating a Fabric peer node
type UpdateFabricPeerOpts struct {
	Mode                    string
	NodeID                  int64
	ExternalEndpoint        string
	ListenAddress           string
	EventsAddress           string
	OperationsListenAddress string
	ChaincodeAddress        string
	DomainNames             []string
	Env                     map[string]string
	AddressOverrides        []types.AddressOverride
	Version                 string
}

// UpdateFabricOrdererOpts represents the options for updating a Fabric orderer node
type UpdateFabricOrdererOpts struct {
	Mode                    string
	NodeID                  int64
	ExternalEndpoint        string
	ListenAddress           string
	AdminAddress            string
	OperationsListenAddress string
	DomainNames             []string
	Env                     map[string]string
	Version                 string
}
