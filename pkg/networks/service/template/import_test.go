package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
)

func TestBuildBesuNetworkConfig(t *testing.T) {
	s := &TemplateService{}

	keyID1 := int64(1)
	keyID2 := int64(2)

	vc := NewVariableContext()
	vc.Set("validator1", &ResolvedVariable{
		Name:          "validator1",
		Type:          TypeKey,
		ResolvedKeyID: &keyID1,
	})
	vc.Set("validator2", &ResolvedVariable{
		Name:          "validator2",
		Type:          TypeKey,
		ResolvedKeyID: &keyID2,
	})

	req := &ImportTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
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
						"0xaddr": {Balance: "0x100"},
					},
					ValidatorRefs: []ValidatorRef{
						{VariableRef: "validator1"},
						{VariableRef: "validator2"},
					},
				},
			},
		},
	}

	config, err := s.buildBesuNetworkConfig(nil, req, vc)
	require.NoError(t, err)

	assert.Equal(t, types.NetworkTypeBesu, config.BaseNetworkConfig.Type)
	assert.Equal(t, int64(1337), config.ChainID)
	assert.Equal(t, types.BesuConsensusType("qbft"), config.Consensus)
	assert.Equal(t, 5, config.BlockPeriod)
	assert.Equal(t, 30000, config.EpochLength)
	assert.Equal(t, "0x1fffffffffffff", config.GasLimit)
	assert.Len(t, config.Alloc, 1)
	assert.Equal(t, "0x100", config.Alloc["0xaddr"].Balance)
	assert.Equal(t, []int64{1, 2}, config.InitialValidatorKeyIds)
}

func TestBuildBesuNetworkConfig_MissingValidator(t *testing.T) {
	s := &TemplateService{}

	vc := NewVariableContext()
	// validator1 not set

	req := &ImportTemplateRequest{
		Template: NetworkTemplate{
			Network: NetworkDefinition{
				Platform: "besu",
				Besu: &BesuNetworkTemplate{
					ChainID:   1337,
					Consensus: "qbft",
					ValidatorRefs: []ValidatorRef{
						{VariableRef: "validator1"},
					},
				},
			},
		},
	}

	_, err := s.buildBesuNetworkConfig(nil, req, vc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validator1")
}

func TestBuildFabricNetworkConfig(t *testing.T) {
	s := &TemplateService{}

	orgID := int64(1)
	nodeID := int64(10)
	ordOrgID := int64(2)
	ordNodeID := int64(20)

	vc := NewVariableContext()
	vc.Set("peerOrg1", &ResolvedVariable{
		Name:          "peerOrg1",
		Type:          TypeOrganization,
		ResolvedOrgID: &orgID,
		Properties:    map[string]interface{}{"mspId": "Org1MSP"},
	})
	vc.Set("peerOrg1_peer1", &ResolvedVariable{
		Name:           "peerOrg1_peer1",
		Type:           TypeNode,
		ResolvedNodeID: &nodeID,
	})
	vc.Set("ordererOrg1", &ResolvedVariable{
		Name:          "ordererOrg1",
		Type:          TypeOrganization,
		ResolvedOrgID: &ordOrgID,
		Properties:    map[string]interface{}{"mspId": "OrdererMSP"},
	})
	vc.Set("ordererOrg1_orderer1", &ResolvedVariable{
		Name:           "ordererOrg1_orderer1",
		Type:           TypeNode,
		ResolvedNodeID: &ordNodeID,
	})

	resolver := &VariableResolver{}

	req := &ImportTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{
				Name:     "test-fabric",
				Platform: "fabric",
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
					ApplicationPolicies: map[string]PolicyTemplate{
						"Endorsement": {Type: "Signature", Rule: "OR('${peerOrg1.mspId}.member')"},
					},
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
				CreateNodeVariable("peerOrg1_peer1", "Peer", []string{"fabric"}, false),
				CreateOrganizationVariable("ordererOrg1", "Orderer org", []string{"fabric"}),
				CreateNodeVariable("ordererOrg1_orderer1", "Orderer", []string{"fabric"}, true),
			},
		},
	}

	config, err := s.buildFabricNetworkConfig(nil, req, vc, resolver)
	require.NoError(t, err)

	assert.Equal(t, types.NetworkTypeFabric, config.BaseNetworkConfig.Type)
	assert.Equal(t, "mychannel", config.ChannelName)
	assert.Equal(t, "etcdraft", config.ConsensusType)
	assert.Equal(t, "2s", config.BatchTimeout)
	assert.NotNil(t, config.BatchSize)
	assert.Equal(t, uint32(10), config.BatchSize.MaxMessageCount)

	// Check orgs
	assert.Len(t, config.PeerOrganizations, 1)
	assert.Equal(t, int64(1), config.PeerOrganizations[0].ID)
	assert.Equal(t, []int64{10}, config.PeerOrganizations[0].NodeIDs)

	assert.Len(t, config.OrdererOrganizations, 1)
	assert.Equal(t, int64(2), config.OrdererOrganizations[0].ID)
	assert.Equal(t, []int64{20}, config.OrdererOrganizations[0].NodeIDs)

	// Check policy substitution
	assert.Equal(t, "OR('Org1MSP.member')", config.ApplicationPolicies["Endorsement"].Rule)
}

func TestBuildFabricNetworkConfig_OptionalNodeSkipped(t *testing.T) {
	s := &TemplateService{}

	orgID := int64(1)
	ordOrgID := int64(2)
	ordNodeID := int64(20)

	vc := NewVariableContext()
	vc.Set("peerOrg1", &ResolvedVariable{
		Name:          "peerOrg1",
		Type:          TypeOrganization,
		ResolvedOrgID: &orgID,
		Properties:    map[string]interface{}{"mspId": "Org1MSP"},
	})
	// peerOrg1_peer1 intentionally NOT set (optional)
	vc.Set("ordererOrg1", &ResolvedVariable{
		Name:          "ordererOrg1",
		Type:          TypeOrganization,
		ResolvedOrgID: &ordOrgID,
		Properties:    map[string]interface{}{"mspId": "OrdererMSP"},
	})
	vc.Set("ordererOrg1_orderer1", &ResolvedVariable{
		Name:           "ordererOrg1_orderer1",
		Type:           TypeNode,
		ResolvedNodeID: &ordNodeID,
	})

	resolver := &VariableResolver{}

	req := &ImportTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{
				Platform: "fabric",
				Fabric: &FabricNetworkTemplate{
					ChannelName:   "mychannel",
					ConsensusType: "etcdraft",
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
				CreateNodeVariable("peerOrg1_peer1", "Peer", []string{"fabric"}, false), // optional
				CreateOrganizationVariable("ordererOrg1", "Orderer org", []string{"fabric"}),
				CreateNodeVariable("ordererOrg1_orderer1", "Orderer", []string{"fabric"}, true),
			},
		},
	}

	config, err := s.buildFabricNetworkConfig(nil, req, vc, resolver)
	require.NoError(t, err)

	// Optional peer node skipped
	assert.Len(t, config.PeerOrganizations[0].NodeIDs, 0)
	// Required orderer node present
	assert.Len(t, config.OrdererOrganizations[0].NodeIDs, 1)
}

func TestConvertTemplatePolicies(t *testing.T) {
	policies := map[string]PolicyTemplate{
		"Readers": {Type: "ImplicitMeta", Rule: "ANY Readers"},
		"Admins":  {Type: "ImplicitMeta", Rule: "MAJORITY Admins"},
	}

	result := convertTemplatePolicies(policies)
	assert.Len(t, result, 2)
	assert.Equal(t, "ImplicitMeta", result["Readers"].Type)
	assert.Equal(t, "ANY Readers", result["Readers"].Rule)
}

func TestConvertTemplatePolicies_Nil(t *testing.T) {
	result := convertTemplatePolicies(nil)
	assert.Nil(t, result)
}
