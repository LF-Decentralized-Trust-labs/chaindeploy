package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"io"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/fabric/networkconfig"
	"github.com/chainlaunch/chainlaunch/pkg/fabric/policydsl"
	"github.com/hyperledger/fabric-admin-sdk/pkg/chaincode"
	"github.com/hyperledger/fabric-admin-sdk/pkg/identity"
	"github.com/hyperledger/fabric-admin-sdk/pkg/network"
	gwidentity "github.com/hyperledger/fabric-gateway/pkg/identity"
	pb "github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/spf13/cobra"
)

type installCmd struct {
	chaincode        string
	channel          string
	networkConfigs   []string
	users            []string
	organizations    []string
	signaturePolicy  string
	chaincodeAddress string
	envFile          string
	metaInfPath      string
	pdcFile          string
	local            bool
	rootCert         string
	clientCert       string
	clientKey        string
	logger           *logger.Logger
}

func (c *installCmd) getPeerAndIdentityForOrg(nc *networkconfig.NetworkConfig, org string, peerID string, userID string) (*grpc.ClientConn, identity.SigningIdentity, error) {
	peerConfig, ok := nc.Peers[peerID]
	if !ok {
		return nil, nil, fmt.Errorf("peer %s not found in network config", peerID)
	}
	conn, err := c.getPeerConnection(peerConfig.URL, peerConfig.TLSCACerts.PEM)
	if err != nil {
		return nil, nil, err
	}
	orgConfig, ok := nc.Organizations[org]
	if !ok {
		return nil, nil, fmt.Errorf("organization %s not found in network config", org)
	}
	user, ok := orgConfig.Users[userID]
	if !ok {
		return nil, nil, fmt.Errorf("user %s not found in network config", userID)
	}

	// Get user certificate
	userCert, err := gwidentity.CertificateFromPEM([]byte(user.Cert.PEM))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to read user certificate for user %s and org %s", userID, org)
	}

	// Get user private key
	userPrivateKey, err := gwidentity.PrivateKeyFromPEM([]byte(user.Key.PEM))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to read user private key for user %s and org %s", userID, org)
	}
	// Log certificate and private key PEM content for debugging
	c.logger.Infof("User certificate for %s in org %s: %s", userID, org, user.Cert.PEM)
	c.logger.Infof("User private key for %s in org %s: %s", userID, org, user.Key.PEM)
	userIdentity, err := identity.NewPrivateKeySigningIdentity(org, userCert, userPrivateKey)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create user identity for user %s and org %s", userID, org)
	}
	return conn, userIdentity, nil
}

func (c *installCmd) getPeerConnection(address string, tlsCACert string) (*grpc.ClientConn, error) {
	networkNode := network.Node{
		Addr:          strings.Replace(address, "grpcs://", "", 1),
		TLSCACertByte: []byte(tlsCACert),
	}
	conn, err := network.DialConnection(networkNode)
	if err != nil {
		return nil, fmt.Errorf("failed to dial connection: %w", err)
	}
	return conn, nil

}

func (c installCmd) start() error {
	var chaincodeEndpoint string
	ctx := context.Background()
	if c.local {
		// Use local chaincode address directly
		chaincodeEndpoint = c.chaincodeAddress
	} else {
		return errors.New("a tunnel is not supported in this version")
	}

	label := c.chaincode
	codeTarBytes, err := c.getCodeTarGz(
		chaincodeEndpoint,
		c.rootCert,
		c.clientKey,
		c.clientCert,
		c.metaInfPath,
	)
	if err != nil {
		return err
	}
	pkg, err := c.getChaincodePackage(c.chaincode, codeTarBytes)
	if err != nil {
		return err
	}
	_ = pkg
	packageID := chaincode.GetPackageID(label, pkg)
	c.logger.Infof("packageID: %s", packageID)

	// Load network configs
	networkConfigs := make([]*networkconfig.NetworkConfig, len(c.organizations))
	if len(c.networkConfigs) == 0 {
		return fmt.Errorf("at least one network config file is required")
	}

	// If only one config is provided, use it for all organizations
	if len(c.networkConfigs) == 1 {
		nc, err := networkconfig.LoadFromFile(c.networkConfigs[0])
		if err != nil {
			return err
		}
		for i := range c.organizations {
			networkConfigs[i] = nc
		}
	} else {
		// If multiple configs are provided, validate the count matches organizations
		if len(c.networkConfigs) != len(c.organizations) {
			return fmt.Errorf("number of network configs (%d) must match number of organizations (%d)", len(c.networkConfigs), len(c.organizations))
		}
		// Load each config
		for i, configPath := range c.networkConfigs {
			nc, err := networkconfig.LoadFromFile(configPath)
			if err != nil {
				return fmt.Errorf("failed to load network config %s: %w", configPath, err)
			}
			networkConfigs[i] = nc
		}
	}
	// Log debug information about the installation parameters
	c.logger.Infof("Debug information for chaincode installation:")
	c.logger.Infof("Chaincode: %s", c.chaincode)
	c.logger.Infof("Channel: %s", c.channel)
	c.logger.Infof("Package ID: %s", packageID)

	// Log organizations
	c.logger.Infof("Organizations (%d):", len(c.organizations))
	for i, org := range c.organizations {
		c.logger.Infof("  [%d] %s", i, org)
	}

	// Log network configs
	c.logger.Infof("Network configs (%d):", len(c.networkConfigs))
	for i, configPath := range c.networkConfigs {
		c.logger.Infof("  [%d] %s", i, configPath)
	}

	// Log users
	c.logger.Infof("Users (%d):", len(c.users))
	for i, user := range c.users {
		c.logger.Infof("  [%d] %s", i, user)
	}

	// Log detailed network configuration information
	for i, nc := range networkConfigs {
		orgName := ""
		if i < len(c.organizations) {
			orgName = c.organizations[i]
		}
		c.logger.Infof("Network config [%d] details for org %s:", i, orgName)

		if orgName != "" {
			if orgConfig, ok := nc.Organizations[orgName]; ok {
				c.logger.Infof("  MSPID: %s", orgConfig.MSPID)
				c.logger.Infof("  Peers (%d):", len(orgConfig.Peers))
				for j, peerID := range orgConfig.Peers {
					if peerConfig, ok := nc.Peers[peerID]; ok {
						c.logger.Infof("    [%d] %s - URL: %s", j, peerID, peerConfig.URL)
					} else {
						c.logger.Infof("    [%d] %s - Not found in peers configuration", j, peerID)
					}
				}
				c.logger.Infof("  Users (%d):", len(orgConfig.Users))
				for userName := range orgConfig.Users {
					c.logger.Infof("    %s", userName)
				}
			} else {
				c.logger.Infof("  Organization %s not found in network config", orgName)
			}
		}
	}

	// Log other parameters
	c.logger.Infof("Chaincode address: %s", c.chaincodeAddress)
	c.logger.Infof("Signature policy: %s", c.signaturePolicy)
	c.logger.Infof("Local mode: %v", c.local)
	if c.pdcFile != "" {
		c.logger.Infof("PDC file: %s", c.pdcFile)
	}
	if c.metaInfPath != "" {
		c.logger.Infof("META-INF path: %s", c.metaInfPath)
	}

	// Install chaincode in peers
	for idx, org := range c.organizations {
		nc := networkConfigs[idx]
		orgConfig, ok := nc.Organizations[org]
		if !ok {
			return fmt.Errorf("organization %s not found in network config", org)
		}
		for _, peerID := range orgConfig.Peers {
			peerConfig, ok := nc.Peers[peerID]
			if !ok {
				return fmt.Errorf("peer %s not found in network config", peerID)
			}
			conn, userIdentity, err := c.getPeerAndIdentityForOrg(nc, org, peerID, c.users[idx])
			if err != nil {
				return err
			}
			defer conn.Close()
			peerClient := chaincode.NewPeer(conn, userIdentity)
			result, err := peerClient.Install(ctx, bytes.NewReader(pkg))
			if err != nil && !strings.Contains(err.Error(), "chaincode already successfully installed") {
				return errors.Wrapf(err, "failed to install chaincode for user %s and org %s", c.users[idx], org)
			}
			if result != nil {
				c.logger.Infof("Chaincode installed %s in %s", result.PackageId, peerConfig.URL)
			} else {
				c.logger.Infof("Chaincode already installed in %s", peerConfig.URL)
			}
		}
	}

	applicationPolicy, err := chaincode.NewApplicationPolicy(c.signaturePolicy, "")
	if err != nil {
		return err
	}

	version := "1"
	sequence := 1
	allOrgGateways := []*chaincode.Gateway{}
	for idx, org := range c.organizations {
		nc := networkConfigs[idx]
		orgConfig, ok := nc.Organizations[org]
		if !ok {
			return fmt.Errorf("organization %s not found in network config", org)
		}
		if len(orgConfig.Peers) == 0 {
			return fmt.Errorf("organization %s has no peers", org)
		}
		conn, userIdentity, err := c.getPeerAndIdentityForOrg(nc, org, orgConfig.Peers[0], c.users[idx])
		if err != nil {
			return err
		}
		defer conn.Close()
		gateway := chaincode.NewGateway(conn, userIdentity)
		allOrgGateways = append(allOrgGateways, gateway)
	}
	firstGateway := allOrgGateways[0]
	committedCC, err := firstGateway.QueryCommittedWithName(
		ctx,
		c.channel,
		c.chaincode,
	)
	if err != nil {
		c.logger.Warnf("Error when getting commited chaincodes: %v", err)
	}

	var collections []*pb.CollectionConfig
	if c.pdcFile != "" {
		pdcBytes, err := ioutil.ReadFile(c.pdcFile)
		if err != nil {
			return err
		}
		collections, err = getCollectionConfigFromBytes([]byte(pdcBytes))
		if err != nil {
			return err
		}
	}
	c.logger.Infof("Commited CC=%v", committedCC)
	shouldCommit := committedCC == nil
	if committedCC != nil {
		appPolicy := pb.ApplicationPolicy{}
		err = proto.Unmarshal(committedCC.GetValidationParameter(), &appPolicy)
		if err != nil {
			return err
		}
		var signaturePolicyString string
		switch policy := appPolicy.Type.(type) {
		case *pb.ApplicationPolicy_SignaturePolicy:
			signaturePolicyString = policy.SignaturePolicy.String()
		default:
			return errors.Errorf("unsupported policy type %T", policy)
		}
		newSignaturePolicyString := applicationPolicy.String()
		if signaturePolicyString != newSignaturePolicyString {
			c.logger.Infof("Signature policy changed, old=%s new=%s", signaturePolicyString, newSignaturePolicyString)
			shouldCommit = true
		} else {
			c.logger.Infof("Signature policy not changed, signaturePolicy=%s", signaturePolicyString)
		}
		// compare collections
		oldCollections := committedCC.GetCollections().GetConfig()
		newCollections := collections
		if len(oldCollections) != len(newCollections) {
			c.logger.Infof("Collection config changed, old=%d new=%d", len(oldCollections), len(newCollections))
			shouldCommit = true
		} else {
			for idx, oldCollection := range oldCollections {
				oldCollectionPayload := oldCollection.Payload.(*pb.CollectionConfig_StaticCollectionConfig)
				newCollection := newCollections[idx]
				newCollectionPayload := newCollection.Payload.(*pb.CollectionConfig_StaticCollectionConfig)
				if oldCollectionPayload.StaticCollectionConfig.Name != newCollectionPayload.StaticCollectionConfig.Name {
					c.logger.Infof("Collection config changed, old=%s new=%s", oldCollectionPayload.StaticCollectionConfig.Name, newCollectionPayload.StaticCollectionConfig.Name)
					shouldCommit = true
					break
				}
				oldCollectionPolicy := oldCollection.GetStaticCollectionConfig().MemberOrgsPolicy
				newCollectionPolicy := newCollection.GetStaticCollectionConfig().MemberOrgsPolicy
				if oldCollectionPolicy.GetSignaturePolicy().String() != newCollectionPolicy.GetSignaturePolicy().String() {
					c.logger.Infof("Collection config changed, old=%s new=%s", oldCollectionPolicy.GetSignaturePolicy().String(), newCollectionPolicy.GetSignaturePolicy().String())
					shouldCommit = true
					break
				}
			}
		}
	}
	if committedCC != nil {
		if shouldCommit {
			version = committedCC.GetVersion()
			sequence = int(committedCC.GetSequence()) + 1
		} else {
			version = committedCC.GetVersion()
			sequence = int(committedCC.GetSequence())
		}
		c.logger.Infof("Chaincode already committed, version=%s sequence=%d", version, sequence)
	}
	c.logger.Infof("Should commit=%v", shouldCommit)

	chaincodeDef := &chaincode.Definition{
		ChannelName:       c.channel,
		PackageID:         packageID,
		Name:              c.chaincode,
		Version:           version,
		EndorsementPlugin: "escc",
		ValidationPlugin:  "vscc",
		Sequence:          int64(sequence),
		ApplicationPolicy: applicationPolicy,
		InitRequired:      false,
		Collections:       nil,
	}
	for idx, gateway := range allOrgGateways {
		err := gateway.Approve(ctx, chaincodeDef)
		if err != nil {
			if strings.Contains(err.Error(), "redefine uncommitted") {
				c.logger.Infof("Chaincode already committed, org=%s", c.organizations[idx])
			} else {
				c.logger.Errorf("Error when approving chaincode: %v", err)
				return err
			}
		}

		c.logger.Infof("Chaincode approved, org=%s", c.organizations[idx])
	}
	if shouldCommit {
		// commit chaincode in orgs
		err := firstGateway.Commit(
			ctx,
			chaincodeDef,
		)
		if err != nil {
			c.logger.Errorf("Error when committing chaincode: %v", err)
			return err
		}
		c.logger.Infof("Chaincode committed")
	}

	if c.envFile != "" {
		err = os.WriteFile(c.envFile, []byte(fmt.Sprintf(`
CORE_CHAINCODE_ADDRESS=%s
CORE_CHAINCODE_ID=%s
CORE_PEER_TLS_ENABLED=false
`, c.chaincodeAddress, packageID)), 0777)
		if err != nil {
			c.logger.Warn("Failed to write .env file: %s", err)
			return err
		}
	}
	return nil
}

func (c *installCmd) getChaincodePackage(label string, codeTarGz []byte) ([]byte, error) {
	var err error
	metadataJson := fmt.Sprintf(`
{
  "type": "ccaas",
  "label": "%s"
}
`, label)
	// set up the output file
	buf := &bytes.Buffer{}

	// set up the gzip writer
	gw := gzip.NewWriter(buf)
	defer func(gw *gzip.Writer) {
		err := gw.Close()
		if err != nil {
			c.logger.Warnf("gzip.Writer.Close() failed: %s", err)
		}
	}(gw)
	tw := tar.NewWriter(gw)
	defer func(tw *tar.Writer) {
		err := tw.Close()
		if err != nil {
			c.logger.Warnf("tar.Writer.Close() failed: %s", err)
		}
	}(tw)
	header := new(tar.Header)
	header.Name = "metadata.json"
	metadataJsonBytes := []byte(metadataJson)
	header.Size = int64(len(metadataJsonBytes))
	header.Mode = 0777
	err = tw.WriteHeader(header)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(metadataJsonBytes)
	_, err = io.Copy(tw, r)
	if err != nil {
		return nil, err
	}
	headerCode := new(tar.Header)
	headerCode.Name = "code.tar.gz"
	headerCode.Size = int64(len(codeTarGz))
	headerCode.Mode = 0777
	err = tw.WriteHeader(headerCode)
	if err != nil {
		return nil, err
	}
	r = bytes.NewReader(codeTarGz)
	_, err = io.Copy(tw, r)
	if err != nil {
		return nil, err
	}
	err = tw.Close()
	if err != nil {
		return nil, err
	}
	err = gw.Close()
	if err != nil {
		c.logger.Warnf("gzip.Writer.Close() failed: %s", err)
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *installCmd) getCodeTarGz(
	address string,
	rootCert string,
	clientKey string,
	clientCert string,
	metaInfPath string,
) ([]byte, error) {
	var err error
	// Determine if TLS is required based on certificate presence
	tlsRequired := rootCert != ""
	clientAuthRequired := clientCert != "" && clientKey != ""

	// Read certificate files if provided
	var rootCertContent, clientKeyContent, clientCertContent string
	if tlsRequired {
		rootCertBytes, err := os.ReadFile(rootCert)
		if err != nil {
			return nil, fmt.Errorf("failed to read root certificate: %w", err)
		}
		rootCertContent = string(rootCertBytes)
	}

	if clientAuthRequired {
		clientKeyBytes, err := os.ReadFile(clientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to read client key: %w", err)
		}
		clientKeyContent = string(clientKeyBytes)

		clientCertBytes, err := os.ReadFile(clientCert)
		if err != nil {
			return nil, fmt.Errorf("failed to read client certificate: %w", err)
		}
		clientCertContent = string(clientCertBytes)
	}

	connMap := map[string]interface{}{
		"address":              address,
		"dial_timeout":         "10s",
		"tls_required":         tlsRequired,
		"root_cert":            rootCertContent,
		"client_auth_required": clientAuthRequired,
		"client_key":           clientKeyContent,
		"client_cert":          clientCertContent,
	}
	connJsonBytes, err := json.Marshal(connMap)
	if err != nil {
		return nil, err
	}
	c.logger.Debugf("Conn=%s", string(connJsonBytes))
	// set up the output file
	buf := &bytes.Buffer{}
	// set up the gzip writer
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	header := new(tar.Header)
	header.Name = "connection.json"
	header.Size = int64(len(connJsonBytes))
	header.Mode = 0755
	err = tw.WriteHeader(header)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(connJsonBytes)
	_, err = io.Copy(tw, r)
	if err != nil {
		return nil, err
	}
	if metaInfPath != "" {
		src := metaInfPath
		// walk through 3 file in the folder
		err = filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
			// generate tar header
			header, err := tar.FileInfoHeader(fi, file)
			if err != nil {
				return err
			}

			// must provide real name
			// (see https://golang.org/src/archive/tar/common.go?#L626)
			relname, err := filepath.Rel(src, file)
			if err != nil {
				return err
			}
			if relname == "." {
				return nil
			}
			header.Name = "META-INF/" + filepath.ToSlash(relname)

			// write header
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			// if not a dir, write file content
			if !fi.IsDir() {
				data, err := os.Open(file)
				if err != nil {
					return err
				}
				if _, err := io.Copy(tw, data); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	err = tw.Close()
	if err != nil {
		return nil, err
	}
	err = gw.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func NewInstallCmd(logger *logger.Logger) *cobra.Command {
	c := &installCmd{
		logger: logger,
	}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the chaincode",
		Long:  `Install the chaincode`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c.logger.Infof("Installing the chaincode")

			return c.start()
		},
	}

	f := cmd.Flags()
	f.StringVar(&c.chaincode, "chaincode", "", "chaincode name within the channel")
	f.StringVar(&c.channel, "channel", "", "Channel name")
	f.StringArrayVar(&c.networkConfigs, "config", []string{}, "Network config files (one per organization, if only one is provided it will be used for all organizations)")
	f.StringVar(&c.signaturePolicy, "policy", "", "Signature policy for the chaincode")
	f.StringArrayVarP(&c.organizations, "organizations", "o", []string{}, "Organizations to connect to ")
	f.StringArrayVarP(&c.users, "users", "u", []string{}, "Users to use")
	f.StringVar(&c.chaincodeAddress, "chaincodeAddress", "", "address of the local chaincode server, example: localhost:9999")
	f.StringVar(&c.envFile, "envFile", "", ".env file to write the environments variables")
	f.StringVar(&c.pdcFile, "pdc", "", "pdc file json, see examples/pdc.json")
	f.StringVar(&c.metaInfPath, "metaInf", "", "metadata")
	f.StringVar(&c.rootCert, "rootCert", "", "path to the root certificate file")
	f.StringVar(&c.clientCert, "clientCert", "", "path to the client certificate file")
	f.StringVar(&c.clientKey, "clientKey", "", "path to the client key file")
	f.BoolVar(&c.local, "local", false, "Use local chaincode address without ngrok tunnel")
	return cmd
}

type endorsementPolicy struct {
	ChannelConfigPolicy string `json:"channelConfigPolicy,omitempty"`
	SignaturePolicy     string `json:"signaturePolicy,omitempty"`
}

type collectionConfigJson struct {
	Name              string             `json:"name"`
	Policy            string             `json:"policy"`
	RequiredPeerCount *int32             `json:"requiredPeerCount"`
	MaxPeerCount      *int32             `json:"maxPeerCount"`
	BlockToLive       uint64             `json:"blockToLive"`
	MemberOnlyRead    bool               `json:"memberOnlyRead"`
	MemberOnlyWrite   bool               `json:"memberOnlyWrite"`
	EndorsementPolicy *endorsementPolicy `json:"endorsementPolicy,omitempty"`
}

// getCollectionConfig retrieves the collection configuration
// from the supplied byte array; the byte array must contain a
// json-formatted array of collectionConfigJson elements
func getCollectionConfigFromBytes(cconfBytes []byte) ([]*pb.CollectionConfig, error) {
	cconf := &[]collectionConfigJson{}
	err := json.Unmarshal(cconfBytes, cconf)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse the collection configuration")
	}

	ccarray := make([]*pb.CollectionConfig, 0, len(*cconf))
	for _, cconfitem := range *cconf {
		p, err := policydsl.FromString(cconfitem.Policy)
		if err != nil {
			return nil, errors.WithMessagef(err, "invalid policy %s", cconfitem.Policy)
		}

		cpc := &pb.CollectionPolicyConfig{
			Payload: &pb.CollectionPolicyConfig_SignaturePolicy{
				SignaturePolicy: p,
			},
		}

		var ep *pb.ApplicationPolicy
		if cconfitem.EndorsementPolicy != nil {
			signaturePolicy := cconfitem.EndorsementPolicy.SignaturePolicy
			channelConfigPolicy := cconfitem.EndorsementPolicy.ChannelConfigPolicy
			ep, err = getApplicationPolicy(signaturePolicy, channelConfigPolicy)
			if err != nil {
				return nil, errors.WithMessagef(err, "invalid endorsement policy [%#v]", cconfitem.EndorsementPolicy)
			}
		}

		// Set default requiredPeerCount and MaxPeerCount if not specified in json
		requiredPeerCount := int32(0)
		maxPeerCount := int32(1)
		if cconfitem.RequiredPeerCount != nil {
			requiredPeerCount = *cconfitem.RequiredPeerCount
		}
		if cconfitem.MaxPeerCount != nil {
			maxPeerCount = *cconfitem.MaxPeerCount
		}

		cc := &pb.CollectionConfig{
			Payload: &pb.CollectionConfig_StaticCollectionConfig{
				StaticCollectionConfig: &pb.StaticCollectionConfig{
					Name:              cconfitem.Name,
					MemberOrgsPolicy:  cpc,
					RequiredPeerCount: requiredPeerCount,
					MaximumPeerCount:  maxPeerCount,
					BlockToLive:       cconfitem.BlockToLive,
					MemberOnlyRead:    cconfitem.MemberOnlyRead,
					MemberOnlyWrite:   cconfitem.MemberOnlyWrite,
					EndorsementPolicy: ep,
				},
			},
		}

		ccarray = append(ccarray, cc)
	}

	return ccarray, nil
}

func getApplicationPolicy(signaturePolicy, channelConfigPolicy string) (*pb.ApplicationPolicy, error) {
	if signaturePolicy == "" && channelConfigPolicy == "" {
		// no policy, no problem
		return nil, nil
	}

	if signaturePolicy != "" && channelConfigPolicy != "" {
		// mo policies, mo problems
		return nil, errors.New(`cannot specify both "--signature-policy" and "--channel-config-policy"`)
	}

	var applicationPolicy *pb.ApplicationPolicy
	if signaturePolicy != "" {
		signaturePolicyEnvelope, err := policydsl.FromString(signaturePolicy)
		if err != nil {
			return nil, errors.Errorf("invalid signature policy: %s", signaturePolicy)
		}

		applicationPolicy = &pb.ApplicationPolicy{
			Type: &pb.ApplicationPolicy_SignaturePolicy{
				SignaturePolicy: signaturePolicyEnvelope,
			},
		}
	}

	if channelConfigPolicy != "" {
		applicationPolicy = &pb.ApplicationPolicy{
			Type: &pb.ApplicationPolicy_ChannelConfigPolicyReference{
				ChannelConfigPolicyReference: channelConfigPolicy,
			},
		}
	}

	return applicationPolicy, nil
}
