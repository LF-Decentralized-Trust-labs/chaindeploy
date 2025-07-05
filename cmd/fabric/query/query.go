package query

import (
	"fmt"
	"io"
	"math/rand/v2"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/fabric/networkconfig"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/hyperledger/fabric-admin-sdk/pkg/network"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type queryChaincodeCmd struct {
	configPath string
	mspID      string
	userName   string
	channel    string
	chaincode  string
	fcn        string
	args       []string
	logger     *logger.Logger
}

func (c *queryChaincodeCmd) validate() error {
	return nil
}

func (c *queryChaincodeCmd) getPeerAndIdentityForOrg(nc *networkconfig.NetworkConfig, org string, peerID string, userID string) (*grpc.ClientConn, identity.Sign, *identity.X509Identity, error) {
	peerConfig, ok := nc.Peers[peerID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("peer %s not found in network config", peerID)
	}
	conn, err := c.getPeerConnection(peerConfig.URL, peerConfig.TLSCACerts.PEM)
	if err != nil {
		return nil, nil, nil, err
	}
	orgConfig, ok := nc.Organizations[org]
	if !ok {
		return nil, nil, nil, fmt.Errorf("organization %s not found in network config", org)
	}
	user, ok := orgConfig.Users[userID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("user %s not found in network config", userID)
	}
	userCert, err := identity.CertificateFromPEM([]byte(user.Cert.PEM))
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "failed to read user certificate for user %s and org %s", userID, org)
	}
	userPrivateKey, err := identity.PrivateKeyFromPEM([]byte(user.Key.PEM))
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "failed to read user private key for user %s and org %s", userID, org)
	}
	userPK, err := identity.NewPrivateKeySign(userPrivateKey)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "failed to create user identity for user %s and org %s", userID, org)
	}
	userIdentity, err := identity.NewX509Identity(c.mspID, userCert)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "failed to create user identity for user %s and org %s", userID, org)
	}
	return conn, userPK, userIdentity, nil
}

func (c *queryChaincodeCmd) getPeerConnection(address string, tlsCACert string) (*grpc.ClientConn, error) {

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

func (c *queryChaincodeCmd) run(out io.Writer) error {
	networkConfig, err := networkconfig.LoadFromFile(c.configPath)
	if err != nil {
		return err
	}

	orgConfig, ok := networkConfig.Organizations[c.mspID]
	if !ok {
		return fmt.Errorf("organization %s not found", c.mspID)
	}
	_, ok = orgConfig.Users[c.userName]
	if !ok {
		return fmt.Errorf("user %s not found", c.userName)
	}
	peers := orgConfig.Peers
	if len(peers) == 0 {
		return fmt.Errorf("no peers found for organization %s", c.mspID)
	}

	randomIndex := rand.Int() % len(peers)
	peerID := peers[randomIndex]
	c.logger.Infof("Randomly selected peer: %s", peerID)

	conn, userPK, userIdentity, err := c.getPeerAndIdentityForOrg(networkConfig, c.mspID, peerID, c.userName)
	if err != nil {
		return err
	}
	defer conn.Close()

	gatewayFabric, err := client.Connect(userIdentity, client.WithSign(userPK), client.WithClientConnection(conn))
	if err != nil {
		return err
	}
	defer gatewayFabric.Close()

	network := gatewayFabric.GetNetwork(c.channel)
	contract := network.GetContract(c.chaincode)

	result, err := contract.EvaluateTransaction(c.fcn, c.args...)
	if err != nil {
		endorseError, ok := err.(*client.EndorseError)
		if ok {
			detailsStr := []string{}
			for _, detail := range status.Convert(err).Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s\n", detail.GetAddress(), detail.GetMspId(), detail.GetMessage()))

				}
			}
			return fmt.Errorf("failed to submit transaction: %s (gRPC status: %s)",
				endorseError.TransactionError.Error(),
				strings.Join(detailsStr, "\n"))
		}
		statusError := status.Convert(err)
		if statusError != nil {
			detailsStr := []string{}
			for _, detail := range statusError.Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s",
						detail.GetAddress(),
						detail.GetMspId(),
						detail.GetMessage()))
				}
			}
			return fmt.Errorf("failed to submit transaction: %s (gRPC status details: %s)",
				statusError.Message(),
				strings.Join(detailsStr, "\n"))
		}
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	_, err = fmt.Fprint(out, string(result))
	if err != nil {
		return err
	}
	return nil
}

func NewQueryChaincodeCMD(out io.Writer, errOut io.Writer, logger *logger.Logger) *cobra.Command {
	c := &queryChaincodeCmd{
		logger: logger,
	}
	cmd := &cobra.Command{
		Use: "query",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.validate(); err != nil {
				return err
			}
			return c.run(out)
		},
	}
	persistentFlags := cmd.PersistentFlags()
	persistentFlags.StringVarP(&c.mspID, "mspID", "", "", "Org to use invoke the chaincode")
	persistentFlags.StringVarP(&c.userName, "user", "", "", "User name for the transaction")
	persistentFlags.StringVarP(&c.configPath, "config", "", "", "Configuration file for the SDK")
	persistentFlags.StringVarP(&c.channel, "channel", "", "", "Channel name")
	persistentFlags.StringVarP(&c.chaincode, "chaincode", "", "", "Chaincode label")
	persistentFlags.StringVarP(&c.fcn, "fcn", "", "", "Function name")
	persistentFlags.StringArrayVarP(&c.args, "args", "a", []string{}, "Function arguments")
	cmd.MarkPersistentFlagRequired("user")
	cmd.MarkPersistentFlagRequired("mspID")
	cmd.MarkPersistentFlagRequired("config")
	cmd.MarkPersistentFlagRequired("chaincode")
	cmd.MarkPersistentFlagRequired("fcn")
	return cmd
}
