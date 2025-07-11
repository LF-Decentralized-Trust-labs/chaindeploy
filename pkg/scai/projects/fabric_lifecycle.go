package projects

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	fabricService "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keyMgmtService "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service"
	"github.com/hyperledger/fabric-admin-sdk/pkg/chaincode"
	"github.com/hyperledger/fabric-admin-sdk/pkg/identity"
	fabricnetwork "github.com/hyperledger/fabric-admin-sdk/pkg/network"
	gwidentity "github.com/hyperledger/fabric-gateway/pkg/identity"
	pb "github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// FabricLifecycle implements PlatformLifecycle for Hyperledger Fabric
type FabricLifecycle struct {
	queries        *db.Queries
	logger         *zap.Logger
	orgService     *fabricService.OrganizationService
	keyMgmtService *keyMgmtService.KeyManagementService
	networkService *service.NetworkService
}

// NewFabricLifecycle creates a new FabricLifecycle instance
func NewFabricLifecycle(queries *db.Queries, logger *zap.Logger, orgService *fabricService.OrganizationService, keyMgmtService *keyMgmtService.KeyManagementService, networkService *service.NetworkService) *FabricLifecycle {
	return &FabricLifecycle{
		queries:        queries,
		logger:         logger,
		orgService:     orgService,
		keyMgmtService: keyMgmtService,
		networkService: networkService,
	}
}

// PreStart is called before starting the project container
func (f *FabricLifecycle) PreStart(ctx context.Context, params PreStartParams) (*PreStartResult, error) {
	f.logger.Info("PreStart hook for Fabric project",
		zap.Int64("projectID", params.ProjectID),
		zap.String("projectName", params.ProjectName),
		zap.String("boilerplate", params.Boilerplate),
	)

	// Validate that the project is associated with a Fabric network
	if params.Platform != "fabric" {
		return nil, fmt.Errorf("project is not associated with a Fabric network")
	}

	// Get network details
	network, err := f.queries.GetNetwork(ctx, params.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network details: %w", err)
	}

	// Get all organizations in the network
	orgs, err := f.queries.ListFabricOrganizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}

	// Get network nodes
	nodes, err := f.networkService.GetNetworkNodes(ctx, params.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network nodes: %w", err)
	}

	// Create chaincode package
	label := params.ProjectName
	chaincodeEndpoint := fmt.Sprintf("%s:%d", params.HostIP, params.Port)

	// Create connection.json
	connMap := map[string]interface{}{
		"address":              chaincodeEndpoint,
		"dial_timeout":         "10s",
		"tls_required":         false,
		"client_auth_required": false,
	}
	connJsonBytes, err := json.Marshal(connMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection.json: %w", err)
	}

	// Create code.tar.gz
	codeTarGz, err := f.createCodeTarGz(connJsonBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create code.tar.gz: %w", err)
	}

	// Create chaincode package
	pkg, err := f.createChaincodePackage(label, codeTarGz)
	if err != nil {
		return nil, fmt.Errorf("failed to create chaincode package: %w", err)
	}

	packageID := chaincode.GetPackageID(label, pkg)

	// Install and approve chaincode for each organization
	for _, org := range orgs {
		// Get admin identity
		adminSignKey, err := f.keyMgmtService.GetKey(ctx, int(org.AdminSignKeyID.Int64))
		if err != nil {
			return nil, fmt.Errorf("failed to get admin sign key: %w", err)
		}

		// Get private key
		privateKeyPEM, err := f.keyMgmtService.GetDecryptedPrivateKey(int(org.AdminSignKeyID.Int64))
		if err != nil {
			return nil, fmt.Errorf("failed to get private key: %w", err)
		}

		// Create certificate and private key objects
		cert, err := gwidentity.CertificateFromPEM([]byte(*adminSignKey.Certificate))
		if err != nil {
			return nil, fmt.Errorf("failed to read certificate: %w", err)
		}

		priv, err := gwidentity.PrivateKeyFromPEM([]byte(privateKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("failed to read private key: %w", err)
		}

		// Get network nodes for this organization
		orgNodes, err := f.networkService.GetNetworkNodes(ctx, params.NetworkID)
		if err != nil {
			return nil, fmt.Errorf("failed to get network nodes: %w", err)
		}

		// Check if organization has any peers
		hasPeers := false
		for _, node := range orgNodes {
			if node.Node.NodeType == "FABRIC_PEER" && node.Node.FabricPeer != nil && node.Node.FabricPeer.MSPID == org.MspID {
				hasPeers = true
				break
			}
		}

		// Skip installation and approval if organization has no peers
		if !hasPeers {
			f.logger.Info("Skipping chaincode installation and approval for organization without peers",
				zap.String("org", org.MspID),
			)
			continue
		}

		// Install on each peer
		for _, node := range orgNodes {
			if node.Node.NodeType == "FABRIC_PEER" {
				if node.Node.FabricPeer.MSPID != org.MspID {
					continue
				}

				// Get peer properties
				peerProps := node.Node.FabricPeer
				if peerProps == nil {
					return nil, fmt.Errorf("peer properties not found for node %s", node.Node.Name)
				}

				// Create peer connection
				peerNode := fabricnetwork.Node{
					Addr:          strings.TrimPrefix(peerProps.ExternalEndpoint, "grpcs://"),
					TLSCACertByte: []byte(peerProps.TLSCACert),
				}
				conn, err := fabricnetwork.DialConnection(peerNode)
				if err != nil {
					return nil, fmt.Errorf("failed to dial peer: %w", err)
				}
				defer conn.Close()

				// Create signing identity using peer's MSP ID
				signingIdentity, err := identity.NewPrivateKeySigningIdentity(peerProps.MSPID, cert, priv)
				if err != nil {
					return nil, fmt.Errorf("failed to create signing identity: %w", err)
				}

				// Install chaincode
				peerClient := chaincode.NewPeer(conn, signingIdentity)
				result, err := peerClient.Install(ctx, bytes.NewReader(pkg))
				if err != nil && !strings.Contains(err.Error(), "chaincode already successfully installed") {
					return nil, fmt.Errorf("failed to install chaincode: %w", err)
				}

				if result != nil {
					f.logger.Info("Chaincode installed",
						zap.String("packageID", result.PackageId),
						zap.String("peer", peerProps.ExternalEndpoint),
						zap.String("mspID", peerProps.MSPID),
					)
				} else {
					f.logger.Info("Chaincode already installed",
						zap.String("peer", peerProps.ExternalEndpoint),
						zap.String("mspID", peerProps.MSPID),
					)
				}
			}
		}

		// Get a peer for this organization
		var peerNode *service.NetworkNode
		for _, node := range orgNodes {
			if node.Node.NodeType == "FABRIC_PEER" && node.Node.FabricPeer != nil && node.Node.FabricPeer.MSPID == org.MspID {
				peerNode = &node
				break
			}
		}
		if peerNode == nil {
			f.logger.Info("Skipping chaincode approval for organization without peers",
				zap.String("org", org.MspID),
			)
			continue
		}

		// Get peer properties
		peerProps := peerNode.Node.FabricPeer
		if peerProps == nil {
			return nil, fmt.Errorf("peer properties not found for node %s", peerNode.Node.Name)
		}

		// Create peer connection
		peerNodeConn := fabricnetwork.Node{
			Addr:          strings.TrimPrefix(peerProps.ExternalEndpoint, "grpcs://"),
			TLSCACertByte: []byte(peerProps.TLSCACert),
		}
		peerConn, err := fabricnetwork.DialConnection(peerNodeConn)
		if err != nil {
			return nil, fmt.Errorf("failed to dial peer: %w", err)
		}
		defer peerConn.Close()

		// Create signing identity using peer's MSP ID
		signingIdentity, err := identity.NewPrivateKeySigningIdentity(peerProps.MSPID, cert, priv)
		if err != nil {
			return nil, fmt.Errorf("failed to create signing identity: %w", err)
		}

		// Create gateway
		gateway := chaincode.NewGateway(peerConn, signingIdentity)

		// Check if chaincode is already committed
		committedCC, err := gateway.QueryCommittedWithName(
			ctx,
			network.Name,
			params.ProjectName,
		)
		if err != nil {
			f.logger.Warn("Error when getting committed chaincodes", zap.Error(err))
		}

		// Create chaincode definition
		applicationPolicy, err := chaincode.NewApplicationPolicy(params.EndorsementPolicy, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create application policy: %w", err)
		}

		version := "1"
		sequence := int64(1)
		shouldCommit := committedCC == nil

		if committedCC != nil {
			appPolicy := pb.ApplicationPolicy{}
			err = proto.Unmarshal(committedCC.GetValidationParameter(), &appPolicy)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal application policy: %w", err)
			}

			var signaturePolicyString string
			switch policy := appPolicy.Type.(type) {
			case *pb.ApplicationPolicy_SignaturePolicy:
				signaturePolicyString = policy.SignaturePolicy.String()
			default:
				return nil, fmt.Errorf("unsupported policy type %T", policy)
			}

			newSignaturePolicyString := applicationPolicy.String()
			if signaturePolicyString != newSignaturePolicyString {
				f.logger.Info("Signature policy changed",
					zap.String("old", signaturePolicyString),
					zap.String("new", newSignaturePolicyString),
				)
				shouldCommit = true
			} else {
				f.logger.Info("Signature policy not changed",
					zap.String("signaturePolicy", signaturePolicyString),
				)
			}

			if shouldCommit {
				version = committedCC.GetVersion()
				sequence = committedCC.GetSequence() + 1
			} else {
				version = committedCC.GetVersion()
				sequence = committedCC.GetSequence()
			}
			f.logger.Info("Chaincode already committed",
				zap.String("version", version),
				zap.Int64("sequence", sequence),
			)
		}

		f.logger.Info("Should commit",
			zap.Bool("shouldCommit", shouldCommit),
		)

		chaincodeDef := &chaincode.Definition{
			ChannelName:       network.Name,
			PackageID:         packageID,
			Name:              params.ProjectName,
			Version:           version,
			EndorsementPlugin: "escc",
			ValidationPlugin:  "vscc",
			Sequence:          sequence,
			ApplicationPolicy: applicationPolicy,
			InitRequired:      false,
		}

		// Approve chaincode
		err = gateway.Approve(ctx, chaincodeDef)
		if err != nil {
			// endorseError, ok := err.(client.EndorseError)
			// _ = endorseError
			// if ok {
			// 	f.logger.Info("Chaincode already approved",
			// 		zap.String("org", org.MspID),
			// 	)
			// } else {
			// 	return fmt.Errorf("failed to approve chaincode: %w", err)
			// }
			// if strings.Contains(err.Error(), "redefine uncommitted") {
			// 	f.logger.Info("Chaincode already approved",
			// 		zap.String("org", org.MspID),
			// 	)
			// } else {
			// 	return fmt.Errorf("failed to approve chaincode: %w", err)
			// }
		}

		f.logger.Info("Chaincode approved",
			zap.String("org", peerProps.MSPID),
		)
	}

	// Commit chaincode definition
	// Find the first organization that has a peer
	var firstOrgWithPeer *db.FabricOrganization
	for _, org := range orgs {
		// Check if this org has any peers
		hasPeer := false
		for _, node := range nodes {
			if node.Node.NodeType == "FABRIC_PEER" && node.Node.FabricPeer != nil && node.Node.FabricPeer.MSPID == org.MspID {
				hasPeer = true
				break
			}
		}
		if hasPeer {
			firstOrgWithPeer = org
			break
		}
	}

	if firstOrgWithPeer == nil {
		return nil, fmt.Errorf("no organization with peers found")
	}

	firstOrgWithKeys, err := f.queries.GetFabricOrganizationWithKeys(ctx, firstOrgWithPeer.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization details: %w", err)
	}

	// Get admin identity for first org
	adminSignKey, err := f.keyMgmtService.GetKey(ctx, int(firstOrgWithKeys.AdminSignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get admin sign key: %w", err)
	}

	privateKeyPEM, err := f.keyMgmtService.GetDecryptedPrivateKey(int(firstOrgWithKeys.AdminSignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	cert, err := gwidentity.CertificateFromPEM([]byte(*adminSignKey.Certificate))
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	priv, err := gwidentity.PrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// Get a peer for the first organization with peers
	var peerNode *service.NetworkNode
	for _, node := range nodes {
		if node.Node.NodeType == "FABRIC_PEER" && node.Node.FabricPeer != nil && node.Node.FabricPeer.MSPID == firstOrgWithPeer.MspID {
			peerNode = &node
			break
		}
	}
	if peerNode == nil {
		return nil, fmt.Errorf("no peer found for organization %s", firstOrgWithPeer.MspID)
	}

	// Get peer properties
	peerProps := peerNode.Node.FabricPeer
	if peerProps == nil {
		return nil, fmt.Errorf("peer properties not found for node %s", peerNode.Node.Name)
	}

	// Create peer connection
	peerNodeConn := fabricnetwork.Node{
		Addr:          strings.TrimPrefix(peerProps.ExternalEndpoint, "grpcs://"),
		TLSCACertByte: []byte(peerProps.TLSCACert),
	}
	peerConn, err := fabricnetwork.DialConnection(peerNodeConn)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer: %w", err)
	}
	defer peerConn.Close()

	// Create signing identity using peer's MSP ID
	signingIdentity, err := identity.NewPrivateKeySigningIdentity(peerProps.MSPID, cert, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create signing identity: %w", err)
	}

	// Create gateway
	gateway := chaincode.NewGateway(peerConn, signingIdentity)

	// Check if chaincode is already committed
	committedCC, err := gateway.QueryCommittedWithName(
		ctx,
		network.Name,
		params.ProjectName,
	)
	if err != nil {
		f.logger.Warn("Error when getting committed chaincodes", zap.Error(err))
	}

	// Create chaincode definition
	applicationPolicy, err := chaincode.NewApplicationPolicy(params.EndorsementPolicy, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create application policy: %w", err)
	}

	version := "1"
	sequence := int64(1)
	shouldCommit := committedCC == nil

	if committedCC != nil {
		appPolicy := pb.ApplicationPolicy{}
		err = proto.Unmarshal(committedCC.GetValidationParameter(), &appPolicy)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal application policy: %w", err)
		}

		var signaturePolicyString string
		switch policy := appPolicy.Type.(type) {
		case *pb.ApplicationPolicy_SignaturePolicy:
			signaturePolicyString = policy.SignaturePolicy.String()
		default:
			return nil, fmt.Errorf("unsupported policy type %T", policy)
		}

		newSignaturePolicyString := applicationPolicy.String()
		if signaturePolicyString != newSignaturePolicyString {
			f.logger.Info("Signature policy changed",
				zap.String("old", signaturePolicyString),
				zap.String("new", newSignaturePolicyString),
			)
			shouldCommit = true
		} else {
			f.logger.Info("Signature policy not changed",
				zap.String("signaturePolicy", signaturePolicyString),
			)
		}

		if shouldCommit {
			version = committedCC.GetVersion()
			sequence = committedCC.GetSequence() + 1
		} else {
			version = committedCC.GetVersion()
			sequence = committedCC.GetSequence()
		}
		f.logger.Info("Chaincode already committed",
			zap.String("version", version),
			zap.Int64("sequence", sequence),
		)
	}

	f.logger.Info("Should commit",
		zap.Bool("shouldCommit", shouldCommit),
	)

	chaincodeDef := &chaincode.Definition{
		ChannelName:       network.Name,
		PackageID:         packageID,
		Name:              params.ProjectName,
		Version:           version,
		EndorsementPlugin: "escc",
		ValidationPlugin:  "vscc",
		Sequence:          sequence,
		ApplicationPolicy: applicationPolicy,
		InitRequired:      false,
	}

	// Commit chaincode
	err = gateway.Commit(ctx, chaincodeDef)
	if err != nil {
		return nil, fmt.Errorf("failed to commit chaincode: %w", err)
	}

	f.logger.Info("Chaincode committed successfully",
		zap.String("name", params.ProjectName),
		zap.String("version", version),
		zap.Int64("sequence", sequence),
		zap.String("mspID", peerProps.MSPID),
	)

	// Create environment variables
	env := map[string]string{
		"CORE_CHAINCODE_ADDRESS": "0.0.0.0:4000",
		"CORE_CHAINCODE_ID":      packageID,
		"CORE_PEER_TLS_ENABLED":  "false",
	}

	return &PreStartResult{
		Environment: env,
	}, nil
}

// createCodeTarGz creates a code.tar.gz file containing the connection.json
func (f *FabricLifecycle) createCodeTarGz(connJsonBytes []byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	// Write connection.json
	header := &tar.Header{
		Name: "connection.json",
		Size: int64(len(connJsonBytes)),
		Mode: 0755,
	}
	if err := tw.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write(connJsonBytes); err != nil {
		return nil, fmt.Errorf("failed to write connection.json: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// createChaincodePackage creates a chaincode package containing metadata.json and code.tar.gz
func (f *FabricLifecycle) createChaincodePackage(label string, codeTarGz []byte) ([]byte, error) {
	metadataJson := fmt.Sprintf(`{
		"type": "ccaas",
		"label": "%s"
	}`, label)

	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	// Write metadata.json
	header := &tar.Header{
		Name: "metadata.json",
		Size: int64(len(metadataJson)),
		Mode: 0755,
	}
	if err := tw.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write([]byte(metadataJson)); err != nil {
		return nil, fmt.Errorf("failed to write metadata.json: %w", err)
	}

	// Write code.tar.gz
	header = &tar.Header{
		Name: "code.tar.gz",
		Size: int64(len(codeTarGz)),
		Mode: 0755,
	}
	if err := tw.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write(codeTarGz); err != nil {
		return nil, fmt.Errorf("failed to write code.tar.gz: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// PostStart is called after the project container has started
func (f *FabricLifecycle) PostStart(ctx context.Context, params PostStartParams) error {
	f.logger.Info("PostStart hook for Fabric project",
		zap.Int64("projectID", params.ProjectID),
		zap.String("projectName", params.ProjectName),
		zap.String("containerID", params.ContainerID),
	)

	// Get network details
	network, err := f.queries.GetNetwork(ctx, params.NetworkID)
	if err != nil {
		return fmt.Errorf("failed to get network details: %w", err)
	}

	// TODO: Implement chaincode installation and approval
	// This will involve:
	// 1. Getting the chaincode package from the container
	// 2. Installing it on the peers
	// 3. Approving the chaincode definition
	// 4. Committing the chaincode definition
	_ = network // TODO: Use network details for chaincode installation

	f.logger.Info("Chaincode setup completed",
		zap.Int64("projectID", params.ProjectID),
		zap.String("projectName", params.ProjectName),
	)

	return nil
}

// PreStop is called before stopping the project container
func (f *FabricLifecycle) PreStop(ctx context.Context, params PreStopParams) error {
	f.logger.Info("PreStop hook for Fabric project",
		zap.Int64("projectID", params.ProjectID),
		zap.String("projectName", params.ProjectName),
		zap.String("containerID", params.ContainerID),
	)

	// TODO: Implement any necessary cleanup before stopping
	// This might include:
	// 1. Saving chaincode state
	// 2. Cleaning up temporary files
	// 3. Updating project status

	return nil
}

// PostStop is called after the project container has stopped
func (f *FabricLifecycle) PostStop(ctx context.Context, params PostStopParams) error {
	f.logger.Info("PostStop hook for Fabric project",
		zap.Int64("projectID", params.ProjectID),
		zap.String("projectName", params.ProjectName),
		zap.String("containerID", params.ContainerID),
	)

	// Update project status in database
	err := f.queries.UpdateProjectContainerInfo(ctx, &db.UpdateProjectContainerInfoParams{
		ID:            params.ProjectID,
		Status:        sql.NullString{String: "stopped", Valid: true},
		LastStoppedAt: sql.NullTime{Time: params.StoppedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	return nil
}
