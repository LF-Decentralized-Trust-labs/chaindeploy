package template

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNetworkTemplate(t *testing.T) {
	tmpl := NewNetworkTemplate()
	assert.Equal(t, TemplateVersion, tmpl.Version)
	assert.NotEmpty(t, tmpl.ExportedAt)

	// Verify timestamp is valid RFC3339
	_, err := time.Parse(time.RFC3339, tmpl.ExportedAt)
	assert.NoError(t, err)
}

func TestNetworkTemplateJSON_Fabric(t *testing.T) {
	tmpl := &NetworkTemplate{
		Version:    "2.0.0",
		ExportedAt: "2026-03-20T00:00:00Z",
		Network: NetworkDefinition{
			Name:        "test-fabric",
			Description: "A test Fabric network",
			Platform:    "fabric",
			Fabric: &FabricNetworkTemplate{
				ChannelName:   "mychannel",
				ConsensusType: "etcdraft",
				BatchSize: &BatchSizeTemplate{
					MaxMessageCount:   10,
					AbsoluteMaxBytes:  98304,
					PreferredMaxBytes: 524288,
				},
				BatchTimeout:            "2s",
				ChannelCapabilities:     []string{"V2_0"},
				ApplicationCapabilities: []string{"V2_0"},
				OrdererCapabilities:     []string{"V2_0"},
				PeerOrgRefs: []OrganizationRef{
					{VariableRef: "peerOrg1", NodeRefs: []NodeRef{{VariableRef: "peerOrg1_peer1"}}},
				},
				OrdererOrgRefs: []OrganizationRef{
					{VariableRef: "ordererOrg1", NodeRefs: []NodeRef{{VariableRef: "ordererOrg1_orderer1"}}},
				},
			},
		},
		Variables: []TemplateVariable{
			CreateOrganizationVariable("peerOrg1", "Peer org", []string{"fabric"}),
			CreateNodeVariable("peerOrg1_peer1", "Peer node", []string{"fabric"}, false),
			CreateOrganizationVariable("ordererOrg1", "Orderer org", []string{"fabric"}),
			CreateNodeVariable("ordererOrg1_orderer1", "Orderer node", []string{"fabric"}, true),
		},
		Chaincodes: []ChaincodeTemplate{
			{
				Name:     "supplychain",
				Platform: "fabric",
				Fabric: &FabricChaincodeTemplate{
					Version:     "1.0",
					Sequence:    1,
					DockerImage: "ghcr.io/chainlaunch/supplychain-cc:latest",
				},
			},
		},
	}

	// Marshal
	data, err := json.Marshal(tmpl)
	require.NoError(t, err)

	// Unmarshal
	var parsed NetworkTemplate
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "2.0.0", parsed.Version)
	assert.Equal(t, "fabric", parsed.Network.Platform)
	assert.Equal(t, "mychannel", parsed.Network.Fabric.ChannelName)
	assert.Equal(t, "etcdraft", parsed.Network.Fabric.ConsensusType)
	assert.Equal(t, uint32(10), parsed.Network.Fabric.BatchSize.MaxMessageCount)
	assert.Len(t, parsed.Network.Fabric.PeerOrgRefs, 1)
	assert.Len(t, parsed.Network.Fabric.OrdererOrgRefs, 1)
	assert.Len(t, parsed.Variables, 4)
	assert.Len(t, parsed.Chaincodes, 1)
	assert.Equal(t, "supplychain", parsed.Chaincodes[0].Name)
	assert.Equal(t, "1.0", parsed.Chaincodes[0].Fabric.Version)
}

func TestNetworkTemplateJSON_Besu(t *testing.T) {
	tmpl := &NetworkTemplate{
		Version:    "2.0.0",
		ExportedAt: "2026-03-20T00:00:00Z",
		Network: NetworkDefinition{
			Name:     "test-besu",
			Platform: "besu",
			Besu: &BesuNetworkTemplate{
				Consensus:      "qbft",
				ChainID:        1337,
				BlockPeriod:    5,
				EpochLength:    30000,
				RequestTimeout: 10,
				GasLimit:       "0x1fffffffffffff",
				Difficulty:     "0x1",
				Alloc: map[string]AllocTemplate{
					"0xfe3b557e8fb62b89f4916b721be55ceb828dbd73": {Balance: "0x200000000000000000000000000000000000000000000000000000000000000"},
				},
				ValidatorRefs: []ValidatorRef{
					{VariableRef: "validator1"},
					{VariableRef: "validator2"},
				},
			},
		},
		Variables: []TemplateVariable{
			CreateValidatorVariable("validator1", "Validator 1"),
			CreateValidatorVariable("validator2", "Validator 2"),
		},
		Chaincodes: []ChaincodeTemplate{
			{
				Name:     "SimpleToken",
				Platform: "besu",
				Besu: &BesuContractTemplate{
					ABI:      `[{"type":"function","name":"balanceOf"}]`,
					Bytecode: "0x608060405234801561001057600080fd5b50",
				},
			},
		},
	}

	data, err := json.Marshal(tmpl)
	require.NoError(t, err)

	var parsed NetworkTemplate
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "besu", parsed.Network.Platform)
	assert.Equal(t, int64(1337), parsed.Network.Besu.ChainID)
	assert.Equal(t, "qbft", parsed.Network.Besu.Consensus)
	assert.Len(t, parsed.Network.Besu.ValidatorRefs, 2)
	assert.Len(t, parsed.Network.Besu.Alloc, 1)
	assert.Len(t, parsed.Chaincodes, 1)
	assert.Equal(t, "SimpleToken", parsed.Chaincodes[0].Name)
	assert.NotEmpty(t, parsed.Chaincodes[0].Besu.ABI)
}

func TestValidateTemplateRequest_JSON(t *testing.T) {
	req := ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{
				Name:     "test",
				Platform: "besu",
				Besu: &BesuNetworkTemplate{
					Consensus: "qbft",
					ChainID:   1337,
				},
			},
		},
		Overrides: ImportOverrides{},
		VariableBindings: []VariableBinding{
			{VariableName: "val1", ExistingKeyID: intPtr(1)},
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var parsed ValidateTemplateRequest
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "2.0.0", parsed.Template.Version)
	assert.Len(t, parsed.VariableBindings, 1)
	assert.Equal(t, "val1", parsed.VariableBindings[0].VariableName)
	assert.Equal(t, int64(1), *parsed.VariableBindings[0].ExistingKeyID)
}

func TestImportTemplateResponse_JSON(t *testing.T) {
	resp := ImportTemplateResponse{
		NetworkID:   42,
		NetworkName: "my-network",
		CreatedOrganizations: []CreatedOrg{
			{TemplateID: "peerOrg1", OrgID: 1, MspID: "PeerOrg1MSP"},
		},
		CreatedNodes: []CreatedNode{
			{TemplateID: "peer1", NodeID: 10, Name: "peer0", Type: "peer"},
		},
		CreatedChaincodes: []CreatedChaincode{
			{Name: "supplychain", ChaincodeID: 5, Platform: "fabric"},
		},
		Message: "Success",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed ImportTemplateResponse
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, int64(42), parsed.NetworkID)
	assert.Len(t, parsed.CreatedOrganizations, 1)
	assert.Len(t, parsed.CreatedNodes, 1)
	assert.Len(t, parsed.CreatedChaincodes, 1)
	assert.Equal(t, "supplychain", parsed.CreatedChaincodes[0].Name)
}

func intPtr(i int64) *int64 {
	return &i
}
