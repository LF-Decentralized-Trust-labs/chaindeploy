//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"testing"

	"github.com/lithammer/shortuuid/v4"
	"github.com/stretchr/testify/require"

	"github.com/chainlaunch/chainlaunch/pkg/common/ports"
	orgtypes "github.com/chainlaunch/chainlaunch/pkg/fabric/handler"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	nodeshttp "github.com/chainlaunch/chainlaunch/pkg/nodes/http"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// TestCreateNode tests the node creation flow
func TestCreateNode(t *testing.T) {
	client, err := NewTestClient()
	require.NoError(t, err)

	// Generate random names
	orgName := fmt.Sprintf("org-%s", shortuuid.New())
	nodeName := fmt.Sprintf("node-%s", shortuuid.New())
	peerName := fmt.Sprintf("peer-%s", shortuuid.New())
	mspID := fmt.Sprintf("MSP-%s", shortuuid.New())

	// Create a fabric organization
	orgReq := &orgtypes.CreateOrganizationRequest{
		Name:        orgName,
		MspID:       mspID,
		Description: fmt.Sprintf("Description for %s", orgName),
		ProviderID:  1,
	}
	orgResp, err := client.CreateOrganization(orgReq)
	require.NoError(t, err)
	orgID := orgResp.ID
	logger := logger.NewDefault()
	logger.Info("Created organization", "id", orgID)
	// Get free ports for the Fabric peer
	peerPort, err := ports.GetFreePort("fabric-peer")
	require.NoError(t, err)
	defer ports.ReleasePort(peerPort.Port)

	eventsPort, err := ports.GetFreePort("fabric-peer")
	require.NoError(t, err)
	defer ports.ReleasePort(eventsPort.Port)

	operationsPort, err := ports.GetFreePort("fabric-peer")
	require.NoError(t, err)
	defer ports.ReleasePort(operationsPort.Port)

	chaincodePort, err := ports.GetFreePort("fabric-peer")
	require.NoError(t, err)
	defer ports.ReleasePort(chaincodePort.Port)

	// First create a node
	fabricCreateReq := &nodeshttp.CreateNodeRequest{
		Name:               nodeName,
		BlockchainPlatform: "FABRIC",
		FabricPeer: &nodetypes.FabricPeerConfig{
			BaseNodeConfig: nodetypes.BaseNodeConfig{
				Type: "fabric-peer",
				Mode: "docker",
			},
			Name:                    peerName,
			MSPID:                   mspID,
			OrganizationID:          orgID,
			ListenAddress:           fmt.Sprintf("0.0.0.0:%d", peerPort.Port),
			EventsAddress:           fmt.Sprintf("0.0.0.0:%d", eventsPort.Port),
			OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", operationsPort.Port),
			ExternalEndpoint:        fmt.Sprintf("localhost:%d", peerPort.Port),
			ChaincodeAddress:        fmt.Sprintf("localhost:%d", chaincodePort.Port),
			DomainNames:             []string{},
			Env:                     map[string]string{},
			Version:                 "3.1.0",
			OrdererAddressOverrides: []nodetypes.OrdererAddressOverride{},
			AddressOverrides:        []nodetypes.AddressOverride{},
		},
	}

	fabricCreateResp, err := client.CreateNode(fabricCreateReq)
	require.NoError(t, err)
	fabricID := fabricCreateResp.ID
	// Check node status
	require.Equal(t, "RUNNING", fabricCreateResp.Status)	
	// Assert that the fabric node ID is not zero
	require.NotZero(t, fabricID, "Fabric node ID should not be zero")
}
