//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	nethttp "net/http"
	"os"
	"testing"
	"time"

	"github.com/lithammer/shortuuid/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chainlaunch/chainlaunch/pkg/common/ports"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

type templateImportResponse struct {
	NetworkID         int64              `json:"networkId"`
	NetworkName       string             `json:"networkName"`
	CreatedChaincodes []createdChaincode `json:"createdChaincodes"`
	Message           string             `json:"message"`
}

type createdChaincode struct {
	Name        string `json:"name"`
	ChaincodeID int64  `json:"chaincodeId"`
	Platform    string `json:"platform"`
}

type chaincodeDetailResponse struct {
	Chaincode   json.RawMessage       `json:"chaincode"`
	Definitions []chaincodeDefinition `json:"definitions"`
}

type chaincodeDefinition struct {
	ID          int64  `json:"id"`
	Version     string `json:"version"`
	Sequence    int64  `json:"sequence"`
	DockerImage string `json:"docker_image"`
}

func getExternalIP() string {
	if ip := os.Getenv("EXTERNAL_IP"); ip != "" {
		return ip
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			return ip.String()
		}
	}
	return "127.0.0.1"
}

// nodeMode returns "docker" if Docker is available, otherwise "service"
func nodeMode() string {
	if m := os.Getenv("NODE_MODE"); m != "" {
		return m
	}
	return "docker"
}

// waitForNodeRunning polls the node status until RUNNING or timeout
func waitForNodeRunning(t *testing.T, client *TestClient, nodeID int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.DoRequest(nethttp.MethodGet, fmt.Sprintf("/nodes/%d", nodeID), nil)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var node map[string]interface{}
			if json.Unmarshal(body, &node) == nil {
				status, _ := node["status"].(string)
				if status == "RUNNING" {
					t.Logf("  Node %d is RUNNING", nodeID)
					return
				}
				if status == "ERROR" {
					t.Fatalf("Node %d failed to start: %s", nodeID, string(body))
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("Node %d did not reach RUNNING status within %s", nodeID, timeout)
}

// TestTemplateFullLifecycle creates a Fabric network from template,
// starts real nodes (docker mode), joins them, and creates chaincodes.
// Requires: Docker running, server started with dev mode
func TestTemplateFullLifecycle(t *testing.T) {
	client, err := NewTestClient()
	require.NoError(t, err)

	suffix := shortuuid.New()[0:5]
	mode := nodeMode()
	externalIP := getExternalIP()
	t.Logf("Using mode=%s, externalIP=%s", mode, externalIP)

	// --- Step 1: Create organizations ---
	t.Log("Step 1: Creating organizations...")
	peerOrgMSP := fmt.Sprintf("PeerOrg%sMSP", suffix)
	ordererOrgMSP := fmt.Sprintf("OrdererOrg%sMSP", suffix)

	peerOrgID := createOrg(t, client, peerOrgMSP, fmt.Sprintf("PeerOrg-%s", suffix))
	ordererOrgID := createOrg(t, client, ordererOrgMSP, fmt.Sprintf("OrdererOrg-%s", suffix))
	t.Logf("  Peer org ID=%d, Orderer org ID=%d", peerOrgID, ordererOrgID)

	// --- Step 2: Create nodes ---
	t.Log("Step 2: Creating nodes...")
	peerNodeID := createPeerNode(t, client, fmt.Sprintf("peer-%s", suffix), peerOrgID, peerOrgMSP, mode, externalIP)
	t.Logf("  Peer node ID=%d", peerNodeID)

	ordererIDs := make([]int64, 3)
	for i := 0; i < 3; i++ {
		ordererIDs[i] = createOrdererNode(t, client, fmt.Sprintf("ord%d-%s", i, suffix), ordererOrgID, ordererOrgMSP, mode, externalIP)
		t.Logf("  Orderer %d node ID=%d", i, ordererIDs[i])
	}

	// --- Step 3: Wait for nodes to be RUNNING ---
	t.Log("Step 3: Waiting for nodes to start...")
	nodeTimeout := 90 * time.Second
	waitForNodeRunning(t, client, peerNodeID, nodeTimeout)
	for i, id := range ordererIDs {
		waitForNodeRunning(t, client, id, nodeTimeout)
		t.Logf("  Orderer %d running", i)
	}

	// --- Step 4: Import Fabric network from template with chaincode ---
	t.Log("Step 4: Importing network from template...")
	ccName := fmt.Sprintf("test-cc-%s", suffix)

	ordererNodeRefs := "["
	ordererNodeVars := ""
	ordererBindings := ""
	for i := 0; i < 3; i++ {
		if i > 0 {
			ordererNodeRefs += ","
			ordererNodeVars += ","
			ordererBindings += ","
		}
		vn := fmt.Sprintf("ordererOrg1_orderer%d", i+1)
		ordererNodeRefs += fmt.Sprintf(`{"variableRef":"%s"}`, vn)
		ordererNodeVars += fmt.Sprintf(`{"name":"%s","type":"node","required":true,"scope":"node","platform":["fabric"]}`, vn)
		ordererBindings += fmt.Sprintf(`{"variableName":"%s","existingNodeId":%d}`, vn, ordererIDs[i])
	}
	ordererNodeRefs += "]"

	importBody := fmt.Sprintf(`{
		"template":{
			"version":"2.0.0","exportedAt":"2026-03-20T00:00:00Z",
			"network":{
				"name":"tmpl-%s","description":"Full lifecycle test","platform":"fabric",
				"fabric":{
					"channelName":"testchannel","consensusType":"etcdraft",
					"batchSize":{"maxMessageCount":10,"absoluteMaxBytes":98304,"preferredMaxBytes":524288},
					"batchTimeout":"2s",
					"channelCapabilities":["V2_0"],"applicationCapabilities":["V2_0"],"ordererCapabilities":["V2_0"],
					"peerOrgRefs":[{"variableRef":"peerOrg1","nodeRefs":[{"variableRef":"peerOrg1_peer1"}]}],
					"ordererOrgRefs":[{"variableRef":"ordererOrg1","nodeRefs":%s}]
				}
			},
			"variables":[
				{"name":"peerOrg1","type":"organization","required":true,"scope":"organization","platform":["fabric"],"properties":[{"name":"mspId","type":"mspId","required":true}]},
				{"name":"peerOrg1_peer1","type":"node","required":false,"scope":"node","platform":["fabric"]},
				{"name":"ordererOrg1","type":"organization","required":true,"scope":"organization","platform":["fabric"],"properties":[{"name":"mspId","type":"mspId","required":true}]},
				%s
			],
			"chaincodes":[{"name":"%s","platform":"fabric","fabric":{"version":"1.0","sequence":1,"dockerImage":"ghcr.io/chainlaunch/test-cc:latest","endorsementPolicy":""}}]
		},
		"overrides":{},
		"variableBindings":[
			{"variableName":"peerOrg1","existingOrgId":%d},
			{"variableName":"peerOrg1_peer1","existingNodeId":%d},
			{"variableName":"ordererOrg1","existingOrgId":%d},
			%s
		]
	}`, suffix, ordererNodeRefs, ordererNodeVars, ccName, peerOrgID, peerNodeID, ordererOrgID, ordererBindings)

	resp, err := client.DoRequest(nethttp.MethodPost, "/networks/templates/import", json.RawMessage(importBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, nethttp.StatusCreated, resp.StatusCode, "Import failed: %s", string(body))

	var importResp templateImportResponse
	require.NoError(t, json.Unmarshal(body, &importResp))
	t.Logf("  Network ID=%d, Name=%s", importResp.NetworkID, importResp.NetworkName)

	require.Len(t, importResp.CreatedChaincodes, 1)
	cc := importResp.CreatedChaincodes[0]
	assert.Equal(t, ccName, cc.Name)
	t.Logf("  Chaincode ID=%d, Name=%s", cc.ChaincodeID, cc.Name)

	// --- Step 5: Verify chaincode definition ---
	t.Log("Step 5: Verifying chaincode definition...")
	ccResp, err := client.DoRequest(nethttp.MethodGet, fmt.Sprintf("/sc/fabric/chaincodes/%d", cc.ChaincodeID), nil)
	require.NoError(t, err)
	defer ccResp.Body.Close()
	ccBody, _ := io.ReadAll(ccResp.Body)
	require.Equal(t, nethttp.StatusOK, ccResp.StatusCode)

	var ccDetail chaincodeDetailResponse
	require.NoError(t, json.Unmarshal(ccBody, &ccDetail))
	require.Len(t, ccDetail.Definitions, 1)
	assert.Equal(t, "1.0", ccDetail.Definitions[0].Version)
	assert.Equal(t, int64(1), ccDetail.Definitions[0].Sequence)
	assert.Equal(t, "ghcr.io/chainlaunch/test-cc:latest", ccDetail.Definitions[0].DockerImage)
	t.Logf("  Definition ID=%d, Version=%s, Image=%s", ccDetail.Definitions[0].ID, ccDetail.Definitions[0].Version, ccDetail.Definitions[0].DockerImage)

	// --- Step 6: Join nodes to network ---
	t.Log("Step 6: Joining nodes to network...")
	for i, ordID := range ordererIDs {
		joinResp, err := client.DoRequest(nethttp.MethodPost,
			fmt.Sprintf("/networks/fabric/%d/orderers/%d/join", importResp.NetworkID, ordID), nil)
		require.NoError(t, err)
		joinBody, _ := io.ReadAll(joinResp.Body)
		joinResp.Body.Close()
		t.Logf("  Orderer %d join: %d - %s", i, joinResp.StatusCode, truncate(string(joinBody), 200))
		require.Equal(t, nethttp.StatusOK, joinResp.StatusCode, "Orderer %d join failed: %s", i, string(joinBody))
	}

	// Wait a moment for orderers to form consensus
	time.Sleep(5 * time.Second)

	joinResp, err := client.DoRequest(nethttp.MethodPost,
		fmt.Sprintf("/networks/fabric/%d/peers/%d/join", importResp.NetworkID, peerNodeID), nil)
	require.NoError(t, err)
	joinBody, _ := io.ReadAll(joinResp.Body)
	joinResp.Body.Close()
	t.Logf("  Peer join: %d - %s", joinResp.StatusCode, truncate(string(joinBody), 200))
	require.Equal(t, nethttp.StatusOK, joinResp.StatusCode, "Peer join failed: %s", string(joinBody))

	// --- Step 7: Install chaincode on peer ---
	t.Log("Step 7: Installing chaincode by definition...")
	defID := ccDetail.Definitions[0].ID
	installResp, err := client.DoRequest(nethttp.MethodPost,
		fmt.Sprintf("/sc/fabric/definitions/%d/install", defID),
		map[string]interface{}{"peer_ids": []int64{peerNodeID}})
	require.NoError(t, err)
	installBody, _ := io.ReadAll(installResp.Body)
	installResp.Body.Close()
	t.Logf("  Install: %d - %s", installResp.StatusCode, truncate(string(installBody), 200))
	require.Equal(t, nethttp.StatusOK, installResp.StatusCode, "Install failed: %s", string(installBody))

	// --- Step 8: Approve chaincode ---
	t.Log("Step 8: Approving chaincode by definition...")
	approveResp, err := client.DoRequest(nethttp.MethodPost,
		fmt.Sprintf("/sc/fabric/definitions/%d/approve", defID),
		map[string]interface{}{"peer_id": peerNodeID})
	require.NoError(t, err)
	approveBody, _ := io.ReadAll(approveResp.Body)
	approveResp.Body.Close()
	t.Logf("  Approve: %d - %s", approveResp.StatusCode, truncate(string(approveBody), 200))
	require.Equal(t, nethttp.StatusOK, approveResp.StatusCode, "Approve failed: %s", string(approveBody))

	// --- Step 9: Commit chaincode ---
	t.Log("Step 9: Committing chaincode by definition...")
	commitResp, err := client.DoRequest(nethttp.MethodPost,
		fmt.Sprintf("/sc/fabric/definitions/%d/commit", defID),
		map[string]interface{}{"peer_id": peerNodeID})
	require.NoError(t, err)
	commitBody, _ := io.ReadAll(commitResp.Body)
	commitResp.Body.Close()
	t.Logf("  Commit: %d - %s", commitResp.StatusCode, truncate(string(commitBody), 200))
	require.Equal(t, nethttp.StatusOK, commitResp.StatusCode, "Commit failed: %s", string(commitBody))

	// --- Step 10: Export and verify round-trip ---
	t.Log("Step 10: Exporting network as template (round-trip)...")
	exportResp, err := client.DoRequest(nethttp.MethodGet,
		fmt.Sprintf("/networks/templates/export/%d", importResp.NetworkID), nil)
	require.NoError(t, err)
	defer exportResp.Body.Close()
	exportBody, _ := io.ReadAll(exportResp.Body)
	require.Equal(t, nethttp.StatusOK, exportResp.StatusCode)

	var exportData map[string]interface{}
	require.NoError(t, json.Unmarshal(exportBody, &exportData))
	tmpl := exportData["template"].(map[string]interface{})
	assert.Equal(t, "2.0.0", tmpl["version"])
	network := tmpl["network"].(map[string]interface{})
	assert.Equal(t, "fabric", network["platform"])
	variables := tmpl["variables"].([]interface{})
	assert.GreaterOrEqual(t, len(variables), 4)

	t.Log("Full lifecycle test COMPLETE!")
}

// TestTemplateImportBesuWithValidatorKeys tests Besu template import with real keys
func TestTemplateImportBesuWithValidatorKeys(t *testing.T) {
	client, err := NewTestClient()
	require.NoError(t, err)

	suffix := shortuuid.New()[0:5]

	t.Log("Creating 4 Besu validator keys...")
	keyIDs := make([]int64, 4)
	for i := 0; i < 4; i++ {
		keyIDs[i] = createBesuValidatorKey(t, client, fmt.Sprintf("val-%s-%d", suffix, i))
		t.Logf("  Validator %d key ID=%d", i, keyIDs[i])
	}

	bindings := "["
	for i, keyID := range keyIDs {
		if i > 0 {
			bindings += ","
		}
		bindings += fmt.Sprintf(`{"variableName":"validator%d","existingKeyId":%d}`, i+1, keyID)
	}
	bindings += "]"

	importBody := fmt.Sprintf(`{
		"template":{
			"version":"2.0.0","exportedAt":"2026-03-20T00:00:00Z",
			"network":{
				"name":"besu-%s","description":"Besu full test","platform":"besu",
				"besu":{
					"consensus":"qbft","chainId":1337,"blockPeriod":5,"epochLength":30000,"requestTimeout":10,
					"gasLimit":"0x1fffffffffffff","difficulty":"0x1",
					"alloc":{"0xfe3b557e8fb62b89f4916b721be55ceb828dbd73":{"balance":"0x200000000000000000000000000000000000000000000000000000000000000"}},
					"validatorRefs":[{"variableRef":"validator1"},{"variableRef":"validator2"},{"variableRef":"validator3"},{"variableRef":"validator4"}]
				}
			},
			"variables":[
				{"name":"validator1","type":"key","required":true,"scope":"validator","platform":["besu"]},
				{"name":"validator2","type":"key","required":true,"scope":"validator","platform":["besu"]},
				{"name":"validator3","type":"key","required":true,"scope":"validator","platform":["besu"]},
				{"name":"validator4","type":"key","required":true,"scope":"validator","platform":["besu"]}
			],
			"chaincodes":[]
		},
		"overrides":{},"variableBindings":%s
	}`, suffix, bindings)

	t.Log("Importing Besu network from template...")
	resp, err := client.DoRequest(nethttp.MethodPost, "/networks/templates/import", json.RawMessage(importBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, nethttp.StatusCreated, resp.StatusCode, "Import failed: %s", string(body))

	var importResp templateImportResponse
	require.NoError(t, json.Unmarshal(body, &importResp))
	assert.NotZero(t, importResp.NetworkID)
	assert.Contains(t, importResp.Message, "4 validators")
	t.Logf("  Network ID=%d, Name=%s", importResp.NetworkID, importResp.NetworkName)

	// Verify network
	netResp, err := client.DoRequest(nethttp.MethodGet, fmt.Sprintf("/networks/besu/%d", importResp.NetworkID), nil)
	require.NoError(t, err)
	defer netResp.Body.Close()
	netBody, _ := io.ReadAll(netResp.Body)
	require.Equal(t, nethttp.StatusOK, netResp.StatusCode)

	var netData map[string]interface{}
	require.NoError(t, json.Unmarshal(netBody, &netData))
	assert.Equal(t, float64(1337), netData["chainId"])
	assert.Equal(t, "genesis_block_created", netData["status"])

	t.Log("Besu template test COMPLETE!")
}

// --- Helpers ---

func createOrg(t *testing.T, client *TestClient, mspID, name string) int64 {
	t.Helper()
	type req struct {
		MspID       string `json:"mspId"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	resp, err := client.DoRequest(nethttp.MethodPost, "/organizations", &req{MspID: mspID, Name: name, Description: "e2e"})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, nethttp.StatusCreated, resp.StatusCode, "Create org: %s", string(body))
	var r map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &r))
	return int64(r["id"].(float64))
}

func createPeerNode(t *testing.T, client *TestClient, name string, orgID int64, mspID, mode, externalIP string) int64 {
	t.Helper()
	listen, err := ports.GetFreePort("fabric-peer")
	require.NoError(t, err)
	cc, err := ports.GetFreePort("fabric-peer")
	require.NoError(t, err)
	events, err := ports.GetFreePort("fabric-peer")
	require.NoError(t, err)
	ops, err := ports.GetFreePort("fabric-peer")
	require.NoError(t, err)
	t.Cleanup(func() {
		ports.ReleasePort(listen.Port)
		ports.ReleasePort(cc.Port)
		ports.ReleasePort(events.Port)
		ports.ReleasePort(ops.Port)
	})

	type createReq struct {
		Name               string                      `json:"name"`
		BlockchainPlatform string                      `json:"blockchainPlatform"`
		FabricPeer         *nodetypes.FabricPeerConfig `json:"fabricPeer"`
	}
	resp, err := client.DoRequest(nethttp.MethodPost, "/nodes", &createReq{
		Name:               name,
		BlockchainPlatform: "FABRIC",
		FabricPeer: &nodetypes.FabricPeerConfig{
			BaseNodeConfig:          nodetypes.BaseNodeConfig{Mode: mode},
			Name:                    name,
			OrganizationID:          orgID,
			MSPID:                   mspID,
			ListenAddress:           fmt.Sprintf("0.0.0.0:%d", listen.Port),
			ChaincodeAddress:        fmt.Sprintf("0.0.0.0:%d", cc.Port),
			EventsAddress:           fmt.Sprintf("0.0.0.0:%d", events.Port),
			OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", ops.Port),
			ExternalEndpoint:        fmt.Sprintf("%s:%d", externalIP, listen.Port),
			DomainNames:             []string{externalIP},
			Env:                     map[string]string{},
			Version:                 "3.1.0",
			OrdererAddressOverrides: []nodetypes.OrdererAddressOverride{},
			AddressOverrides:        []nodetypes.AddressOverride{},
		},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &r), "Create peer: %s", string(body))
	id, ok := r["id"].(float64)
	require.True(t, ok, "Expected id in: %s", string(body))
	return int64(id)
}

func createOrdererNode(t *testing.T, client *TestClient, name string, orgID int64, mspID, mode, externalIP string) int64 {
	t.Helper()
	listen, err := ports.GetFreePort("fabric-orderer")
	require.NoError(t, err)
	admin, err := ports.GetFreePort("fabric-orderer")
	require.NoError(t, err)
	ops, err := ports.GetFreePort("fabric-orderer")
	require.NoError(t, err)
	t.Cleanup(func() {
		ports.ReleasePort(listen.Port)
		ports.ReleasePort(admin.Port)
		ports.ReleasePort(ops.Port)
	})

	type createReq struct {
		Name               string                          `json:"name"`
		BlockchainPlatform string                          `json:"blockchainPlatform"`
		FabricOrderer      *nodetypes.FabricOrdererConfig `json:"fabricOrderer"`
	}
	resp, err := client.DoRequest(nethttp.MethodPost, "/nodes", &createReq{
		Name:               name,
		BlockchainPlatform: "FABRIC",
		FabricOrderer: &nodetypes.FabricOrdererConfig{
			BaseNodeConfig:          nodetypes.BaseNodeConfig{Mode: mode},
			Name:                    name,
			OrganizationID:          orgID,
			MSPID:                   mspID,
			ListenAddress:           fmt.Sprintf("0.0.0.0:%d", listen.Port),
			AdminAddress:            fmt.Sprintf("0.0.0.0:%d", admin.Port),
			OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", ops.Port),
			ExternalEndpoint:        fmt.Sprintf("%s:%d", externalIP, listen.Port),
			DomainNames:             []string{externalIP},
			Env:                     map[string]string{},
			Version:                 "3.1.0",
			AddressOverrides:        []nodetypes.AddressOverride{},
		},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &r), "Create orderer: %s", string(body))
	id, ok := r["id"].(float64)
	require.True(t, ok, "Expected id in: %s", string(body))
	return int64(id)
}

func createBesuValidatorKey(t *testing.T, client *TestClient, name string) int64 {
	t.Helper()
	type keyReq struct {
		Name      string `json:"name"`
		Algorithm string `json:"algorithm"`
		Curve     string `json:"curve"`
	}
	resp, err := client.DoRequest(nethttp.MethodPost, "/keys", &keyReq{Name: name, Algorithm: "EC", Curve: "secp256k1"})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, nethttp.StatusCreated, resp.StatusCode, "Create key: %s", string(body))
	var r map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &r))
	return int64(r["id"].(float64))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
