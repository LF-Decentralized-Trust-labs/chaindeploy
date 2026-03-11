package invoke

import (
	cryptorand "crypto/rand"
	"fmt"
	"io"
	"math/big"
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

type invokeChaincodeCmd struct {
	configPath string
	mspID      string
	userName   string
	channel    string
	chaincode  string
	fcn        string
	args       []string
	logger     *logger.Logger
}

func (c *invokeChaincodeCmd) validate() error {
	return nil
}

func (c *invokeChaincodeCmd) getPeerAndIdentityForOrg(nc *networkconfig.NetworkConfig, org string, peerID string, userID string) (*grpc.ClientConn, identity.Sign, *identity.X509Identity, error) {
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

func (c *invokeChaincodeCmd) getPeerConnection(address string, tlsCACert string) (*grpc.ClientConn, error) {

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

func (c *invokeChaincodeCmd) run(out io.Writer) error {
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
	// Get a random peer from the organization's peers using crypto/rand
	randIdx, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(peers))))
	if err != nil {
		return fmt.Errorf("failed to generate random index: %w", err)
	}
	randomIndex := randIdx.Int64()

	peerID := peers[randomIndex]
	c.logger.Infof("Randomly selected peer: %s", peerID)

	conn, userPK, userIdentity, err := c.getPeerAndIdentityForOrg(networkConfig, c.mspID, peerID, c.userName)
	if err != nil {
		return err
	}
	defer conn.Close()
	gatewayClient, err := client.Connect(userIdentity, client.WithSign(userPK), client.WithClientConnection(conn))
	if err != nil {
		return err
	}
	defer gatewayClient.Close()
	network := gatewayClient.GetNetwork(c.channel)
	contract := network.GetContract(c.chaincode)
	args := [][]byte{}
	for _, arg := range c.args {
		args = append(args, []byte(arg))
	}

	response, err := contract.NewProposal(c.fcn, client.WithBytesArguments(args...))
	if err != nil {
		return errors.Wrapf(err, "failed to create proposal")
	}
	endorseResponse, err := response.Endorse()
	if err != nil {
		return fmt.Errorf("failed to endorse proposal: %s", gatewayErrorDetail(err))
	}
	// Get the chaincode response payload before submitting
	result := endorseResponse.Result()

	commit, err := endorseResponse.Submit()
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %s", gatewayErrorDetail(err))
	}

	// Wait for the transaction to be committed in a block
	commitStatus, err := commit.Status()
	if err != nil {
		return fmt.Errorf("failed to get commit status: %w", err)
	}
	if !commitStatus.Successful {
		return fmt.Errorf("transaction %s failed to commit with status: %s", commitStatus.TransactionID, commitStatus.Code)
	}

	c.logger.Infof("Transaction %s committed in block %d", commitStatus.TransactionID, commitStatus.BlockNumber)

	if len(result) > 0 {
		_, err = fmt.Fprintln(out, string(result))
		if err != nil {
			return err
		}
	}
	return nil

}

// gatewayErrorDetail extracts detailed error information from Fabric Gateway errors,
// including per-peer error details (address, mspId, message).
func gatewayErrorDetail(err error) string {
	var details []string
	for _, detail := range status.Convert(err).Details() {
		if errDetail, ok := detail.(*gateway.ErrorDetail); ok {
			details = append(details, fmt.Sprintf("- address: %s; mspId: %s; message: %s", errDetail.GetAddress(), errDetail.GetMspId(), errDetail.GetMessage()))
		}
	}
	if len(details) > 0 {
		return fmt.Sprintf("%s\n%s", err.Error(), strings.Join(details, "\n"))
	}
	return err.Error()
}

func NewInvokeChaincodeCMD(out io.Writer, errOut io.Writer, logger *logger.Logger) *cobra.Command {
	c := &invokeChaincodeCmd{
		logger: logger,
	}
	cmd := &cobra.Command{
		Use: "invoke",
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
