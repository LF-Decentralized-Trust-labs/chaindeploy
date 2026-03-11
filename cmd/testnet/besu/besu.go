package besu

import (
	"fmt"
	"os"
	"time"

	"github.com/chainlaunch/chainlaunch/cmd/common"
	"github.com/chainlaunch/chainlaunch/pkg/common/addresses"
	"github.com/chainlaunch/chainlaunch/pkg/common/ports"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	"github.com/chainlaunch/chainlaunch/pkg/networks/http"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/lithammer/shortuuid/v4"
	"github.com/spf13/cobra"
)

func generateShortUUID() string {
	return shortuuid.New()[0:5]
}

// BesuTestnetConfig holds the parameters for creating a Besu testnet
type BesuTestnetConfig struct {
	Name       string
	Nodes      int
	Prefix     string
	Mode       string
	Version    string
	ProviderID int
	// Initial account balances in wei (hex format)
	InitialBalances map[string]string
}

// BesuTestnetRunner encapsulates the config and logic for running and validating the Besu testnet command
type BesuTestnetRunner struct {
	Config BesuTestnetConfig
}

// Validate checks the configuration for required fields
func (r *BesuTestnetRunner) Validate() error {
	if r.Config.Name == "" {
		return fmt.Errorf("--name is required")
	}
	if r.Config.Nodes < 1 {
		return fmt.Errorf("--nodes must be at least 1")
	}
	// For QBFT consensus, require at least 4 nodes
	if r.Config.Nodes < 4 {
		return fmt.Errorf("--nodes must be at least 4 for QBFT consensus")
	}
	if r.Config.Mode != "docker" && r.Config.Mode != "service" {
		return fmt.Errorf("--mode must be either 'docker' or 'service'")
	}
	return nil
}

// Run executes the Besu testnet creation logic
func (r *BesuTestnetRunner) Run() error {
	if err := r.Validate(); err != nil {
		return err
	}

	client, err := common.NewClientFromEnv()
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	externalIP, err := addresses.GetExternalIP()
	if err != nil {
		return fmt.Errorf("failed to get external IP: %w", err)
	}

	// 1. Create all keys and collect their IDs
	fmt.Printf("Creating %d validator keys...\n", r.Config.Nodes)
	keyIDs := make([]int64, 0, r.Config.Nodes)
	nodeNames := make([]string, 0, r.Config.Nodes)
	for i := 0; i < r.Config.Nodes; i++ {
		nodeName := fmt.Sprintf("%s-%s-%d", r.Config.Prefix, r.Config.Name, i+1)
		nodeNames = append(nodeNames, nodeName)
		fmt.Printf("  Creating key for node %s...\n", nodeName)
		providerID := r.Config.ProviderID
		isCA := 0
		keyReq := &models.CreateKeyRequest{
			Name:       nodeName + "-key",
			Algorithm:  models.KeyAlgorithmEC,
			ProviderID: &providerID,
			Curve:      func() *models.ECCurve { c := models.ECCurveSECP256K1; return &c }(),
			IsCA:       &isCA,
		}
		keyResp, err := client.CreateKey(keyReq)
		if err != nil {
			return fmt.Errorf("failed to create key for node %s: %w", nodeName, err)
		}
		fmt.Printf("    Key created: ID %d\n", keyResp.ID)
		keyIDs = append(keyIDs, int64(keyResp.ID))
	}

	// 2. Create the Besu network with all key IDs as validators
	fmt.Printf("Creating Besu network '%s' with %d validators...\n", r.Config.Name, len(keyIDs))
	netReq := &http.CreateBesuNetworkRequest{
		Name:        r.Config.Name,
		Description: "",
	}
	netReq.Config.Consensus = "qbft"
	netReq.Config.ChainID = 1337
	netReq.Config.BlockPeriod = 5
	netReq.Config.EpochLength = 30000
	netReq.Config.RequestTimeout = 10
	netReq.Config.InitialValidatorKeyIds = keyIDs
	netReq.Config.GasLimit = "0x29b92700" // 700000000 in hex
	netReq.Config.Difficulty = "0x1"      // numberToHex(1)
	netReq.Config.MixHash = "0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365"
	netReq.Config.Coinbase = "0x0000000000000000000000000000000000000000"
	netReq.Config.Timestamp = fmt.Sprintf("0x%x", time.Now().Unix()) // Current Unix timestamp in hex (seconds)
	netReq.Config.Nonce = "0x0000000000000000"                        // 8-byte padded nonce

	// Set initial account balances if provided
	if r.Config.InitialBalances != nil {
		netReq.Config.Alloc = make(map[string]struct {
			Balance string `json:"balance" validate:"required,hexadecimal"`
		})
		for address, balance := range r.Config.InitialBalances {
			netReq.Config.Alloc[address] = struct {
				Balance string `json:"balance" validate:"required,hexadecimal"`
			}{
				Balance: balance,
			}
		}
	} else {
		netReq.Config.Alloc = map[string]struct {
			Balance string `json:"balance" validate:"required,hexadecimal"`
		}{}
	}

	netResp, err := client.CreateBesuNetwork(netReq)
	if err != nil {
		return fmt.Errorf("failed to create besu network: %w", err)
	}
	fmt.Printf("  Besu network created: ID %d\n", netResp.ID)

	// 3. Create each Besu node, using the corresponding key
	fmt.Printf("Creating %d Besu nodes...\n", r.Config.Nodes)
	nodeIDs := []int64{}
	var firstNodeEnode string
	for i := 0; i < r.Config.Nodes; i++ {
		nodeName := nodeNames[i]
		keyID := keyIDs[i]
		fmt.Printf("  Creating Besu node %s with key ID %d...\n", nodeName, keyID)
		// Allocate ports for Besu node
		rpcPort, err := ports.GetFreePort("besu")
		if err != nil {
			return fmt.Errorf("failed to allocate RPC port for node %s: %w", nodeName, err)
		}
		p2pPort, err := ports.GetFreePort("besu-p2p")
		if err != nil {
			return fmt.Errorf("failed to allocate P2P port for node %s: %w", nodeName, err)
		}
		metricsPort, err := ports.GetFreePort("besu-metrics")
		if err != nil {
			return fmt.Errorf("failed to allocate metrics port for node %s: %w", nodeName, err)
		}

		bootNodes := []string{}
		if i > 0 {
			bootNodes = []string{firstNodeEnode}
		}

		// Prepare Besu node config (service layer struct)
		besuNodeConfig := &types.BesuNodeConfig{
			BaseNodeConfig:  types.BaseNodeConfig{Mode: r.Config.Mode, Type: "besu"},
			NetworkID:       int64(netResp.ID),
			KeyID:           keyID,
			P2PPort:         uint(p2pPort.Port),
			RPCPort:         uint(rpcPort.Port),
			P2PHost:         "0.0.0.0",
			RPCHost:         "0.0.0.0",
			ExternalIP:      externalIP,
			InternalIP:      externalIP,
			Env:             map[string]string{},
			BootNodes:       bootNodes,
			MetricsEnabled:  true,
			MetricsPort:     int64(metricsPort.Port),
			MetricsProtocol: "PROMETHEUS",
			Version:         r.Config.Version,
		}
		nodeResp, err := client.CreateBesuNode(nodeName, besuNodeConfig)
		if err != nil {
			return fmt.Errorf("failed to create besu node %s: %w", nodeName, err)
		}
		if i == 0 {
			firstNodeEnode = nodeResp.BesuNode.EnodeURL
		}
		fmt.Printf("    Node created ID: %d\n", nodeResp.ID)
		nodeIDs = append(nodeIDs, nodeResp.ID)
	}

	fmt.Printf("Besu testnet created successfully! Network ID: %d\n", netResp.ID)
	return nil
}

func NewBesuTestnetCmd() *cobra.Command {
	runner := &BesuTestnetRunner{
		Config: BesuTestnetConfig{
			InitialBalances: make(map[string]string),
		},
	}

	cmd := &cobra.Command{
		Use:   "besu",
		Short: "Create a Besu testnet",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runner.Run(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&runner.Config.Name, "name", "", "Name of the testnet (required)")
	cmd.Flags().IntVar(&runner.Config.Nodes, "nodes", 1, "Number of nodes (default 1)")
	cmd.Flags().StringVar(&runner.Config.Prefix, "prefix", "besu", "Prefix for node names")
	cmd.Flags().StringVar(&runner.Config.Mode, "mode", "service", "Node mode (service or docker)")
	cmd.Flags().StringVar(&runner.Config.Version, "version", "25.5.0", "Besu version (default 25.5.0)")
	cmd.Flags().StringToStringVar(&runner.Config.InitialBalances, "initial-balance", map[string]string{}, "Initial account balances in wei (hex format), e.g. '0x1234...=0x1000000000000000000'")
	cmd.Flags().IntVar(&runner.Config.ProviderID, "provider-id", 1, "Key provider ID to use for key creation")

	return cmd
}
