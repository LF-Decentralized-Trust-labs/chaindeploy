package fabric

import (
	"fmt"
	"net"
	"os"

	"github.com/chainlaunch/chainlaunch/cmd/common"
	"github.com/chainlaunch/chainlaunch/pkg/common/ports"
	fabrictypes "github.com/chainlaunch/chainlaunch/pkg/fabric/handler"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"

	networkshttp "github.com/chainlaunch/chainlaunch/pkg/networks/http"
	shortuuid "github.com/lithammer/shortuuid/v4"
	"github.com/spf13/cobra"
)

func generateShortUUID() string {
	return shortuuid.New()[0:5]
}

// getExternalIP returns the first non-loopback IPv4 address found on the host
func getExternalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
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
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("no external IP found")
}

// FabricTestnetConfig holds the parameters for creating a Fabric testnet
type FabricTestnetConfig struct {
	Name          string
	Nodes         int
	Org           string
	PeerOrgs      []string
	OrdererOrgs   []string
	Channels      []string
	PeerCounts    map[string]int
	OrdererCounts map[string]int
	Mode          string
	ExternalIP    string
}

// FabricTestnetRunner encapsulates the config and logic for running and validating the Fabric testnet command
type FabricTestnetRunner struct {
	Config FabricTestnetConfig
}

// Validate checks the configuration for required fields
func (r *FabricTestnetRunner) Validate() error {
	if r.Config.Name == "" {
		return fmt.Errorf("--name is required")
	}
	if r.Config.Org == "" {
		return fmt.Errorf("--org is required for fabric")
	}
	// Add more validation as needed

	// Ensure at least 3 orderers in total for consenters
	totalOrderers := 0
	for _, count := range r.Config.OrdererCounts {
		totalOrderers += count
	}
	if totalOrderers < 3 {
		return fmt.Errorf("at least 3 orderers are required in total for consenters (got %d)", totalOrderers)
	}

	return nil
}

// Run executes the Fabric testnet creation logic
func (r *FabricTestnetRunner) Run() error {
	if err := r.Validate(); err != nil {
		return err
	}

	client, err := common.NewClientFromEnv()
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// 1. Create organizations
	orgIDs := map[string]int64{}
	orgNamesWithUUID := map[string]string{}
	for _, org := range r.Config.PeerOrgs {
		orgReq := fabrictypes.CreateOrganizationRequest{Name: org, MspID: org, ProviderID: 1}
		resp, err := client.CreateOrganization(orgReq)
		if err != nil {
			return fmt.Errorf("failed to create peer org %s: %w", org, err)
		}
		orgIDs[org] = resp.ID
		orgNamesWithUUID[org] = org
	}
	for _, org := range r.Config.OrdererOrgs {
		orgReq := fabrictypes.CreateOrganizationRequest{Name: org, MspID: org, ProviderID: 1}
		resp, err := client.CreateOrganization(orgReq)
		if err != nil {
			return fmt.Errorf("failed to create orderer org %s: %w", org, err)
		}
		orgIDs[org] = resp.ID
		orgNamesWithUUID[org] = org
	}

	// 2. Create nodes for each org using common helpers
	nodeIDs := []int64{}
	peerNodeIDs := []int64{}
	ordererNodeIDs := []int64{}
	peerNodeIDsByOrg := map[string][]int64{}
	ordererNodeIDsByOrg := map[string][]int64{}
	for org, count := range r.Config.PeerCounts {
		orgID := orgIDs[org]
		for i := 0; i < count; i++ {
			nodeName := fmt.Sprintf("%s-peer-%s", r.Config.Name, generateShortUUID())

			// Allocate ports for peer node with error handling
			listen, err := ports.GetFreePort("fabric-peer")
			if err != nil {
				return fmt.Errorf("failed to allocate listen port for peer %s: %w", nodeName, err)
			}
			chaincode, err := ports.GetFreePort("fabric-peer")
			if err != nil {
				return fmt.Errorf("failed to allocate chaincode port for peer %s: %w", nodeName, err)
			}
			events, err := ports.GetFreePort("fabric-peer")
			if err != nil {
				return fmt.Errorf("failed to allocate events port for peer %s: %w", nodeName, err)
			}
			operations, err := ports.GetFreePort("fabric-peer")
			if err != nil {
				return fmt.Errorf("failed to allocate operations port for peer %s: %w", nodeName, err)
			}

			// Determine external endpoint based on mode and user preference
			externalIP := "127.0.0.1"
			if r.Config.ExternalIP != "" {
				// Use user-specified external IP
				externalIP = r.Config.ExternalIP
			} else if r.Config.Mode == "docker" {
				// Auto-detect external IP for docker mode
				hostIP, err := getExternalIP()
				if err == nil {
					externalIP = hostIP
				} else {
					// fallback to 127.0.0.1 if error
				}
			}

			peerConfig := &types.FabricPeerConfig{
				Name:           nodeName,
				OrganizationID: orgID,
				BaseNodeConfig: types.BaseNodeConfig{
					Mode: r.Config.Mode,
				},
				MSPID:                   org,
				ListenAddress:           fmt.Sprintf("0.0.0.0:%d", listen.Port),
				ChaincodeAddress:        fmt.Sprintf("0.0.0.0:%d", chaincode.Port),
				EventsAddress:           fmt.Sprintf("0.0.0.0:%d", events.Port),
				OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", operations.Port),
				ExternalEndpoint:        fmt.Sprintf("%s:%d", externalIP, listen.Port),
				DomainNames:             []string{externalIP},
				Env:                     map[string]string{},
				Version:                 "3.1.0",
				AddressOverrides:        []types.AddressOverride{},
				OrdererAddressOverrides: []types.OrdererAddressOverride{},
			}
			nodeResp, err := client.CreatePeerNode(peerConfig)
			if err != nil {
				return fmt.Errorf("failed to create peer node for org %s: %w", org, err)
			}
			nodeIDs = append(nodeIDs, nodeResp.ID)
			peerNodeIDs = append(peerNodeIDs, nodeResp.ID)
			peerNodeIDsByOrg[org] = append(peerNodeIDsByOrg[org], nodeResp.ID)
		}
	}
	for org, count := range r.Config.OrdererCounts {
		orgID := orgIDs[org]
		for i := 0; i < count; i++ {
			nodeName := fmt.Sprintf("%s-orderer-%s", r.Config.Name, generateShortUUID())

			// Allocate ports for orderer node with error handling
			listen, err := ports.GetFreePort("fabric-orderer")
			if err != nil {
				return fmt.Errorf("failed to allocate listen port for orderer %s: %w", nodeName, err)
			}
			admin, err := ports.GetFreePort("fabric-orderer")
			if err != nil {
				return fmt.Errorf("failed to allocate admin port for orderer %s: %w", nodeName, err)
			}
			operations, err := ports.GetFreePort("fabric-orderer")
			if err != nil {
				return fmt.Errorf("failed to allocate operations port for orderer %s: %w", nodeName, err)
			}

			// Determine external endpoint based on mode and user preference
			externalIP := "127.0.0.1"
			if r.Config.ExternalIP != "" {
				// Use user-specified external IP
				externalIP = r.Config.ExternalIP
			} else if r.Config.Mode == "docker" {
				// Auto-detect external IP for docker mode
				hostIP, err := getExternalIP()
				if err == nil {
					externalIP = hostIP
				} else {
					// fallback to 127.0.0.1 if error
				}
			}

			ordererConfig := &types.FabricOrdererConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Mode: r.Config.Mode,
				},
				Name:                    nodeName,
				OrganizationID:          orgID,
				MSPID:                   org,
				ListenAddress:           fmt.Sprintf("0.0.0.0:%d", listen.Port),
				AdminAddress:            fmt.Sprintf("0.0.0.0:%d", admin.Port),
				OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", operations.Port),
				ExternalEndpoint:        fmt.Sprintf("%s:%d", externalIP, listen.Port),
				DomainNames:             []string{externalIP},
				Env:                     map[string]string{},
				Version:                 "3.1.0",
				AddressOverrides:        []types.AddressOverride{},
			}
			nodeResp, err := client.CreateOrdererNode(ordererConfig)
			if err != nil {
				return fmt.Errorf("failed to create orderer node for org %s: %w", org, err)
			}
			nodeIDs = append(nodeIDs, nodeResp.ID)
			ordererNodeIDs = append(ordererNodeIDs, nodeResp.ID)
			ordererNodeIDsByOrg[org] = append(ordererNodeIDsByOrg[org], nodeResp.ID)
		}
	}

	// 3. Create the network/channels using the common helper
	// Build the FabricNetworkConfig
	peerOrgs := []networkshttp.OrganizationConfig{}
	ordererOrgs := []networkshttp.OrganizationConfig{}
	for _, org := range r.Config.PeerOrgs {
		peerOrgs = append(peerOrgs, networkshttp.OrganizationConfig{
			ID:      orgIDs[org],
			NodeIDs: peerNodeIDsByOrg[org],
		})
	}
	for _, org := range r.Config.OrdererOrgs {
		ordererOrgs = append(ordererOrgs, networkshttp.OrganizationConfig{
			ID:      orgIDs[org],
			NodeIDs: ordererNodeIDsByOrg[org],
		})
	}

	netReq := &networkshttp.CreateFabricNetworkRequest{
		Name:        r.Config.Name,
		Description: "",
		Config: networkshttp.FabricNetworkConfig{
			PeerOrganizations:    peerOrgs,
			OrdererOrganizations: ordererOrgs,
		},
	}

	networkResp, err := client.CreateFabricNetwork(netReq)
	if err != nil {
		return fmt.Errorf("failed to create fabric network: %w", err)
	}

	fmt.Printf("Fabric testnet created successfully! Network ID: %d\n", networkResp.ID)

	// Join all peers to the network
	peerResults, peerErrs := client.JoinAllPeersToFabricNetwork(networkResp.ID, peerNodeIDs)
	for _, resp := range peerResults {
		fmt.Printf("Peer joined network %d successfully. Network ID: %d, Status: %s\n", networkResp.ID, resp.ID, resp.Status)
	}
	if len(peerErrs) > 0 {
		fmt.Println("Errors occurred while joining some peers:")
		for _, err := range peerErrs {
			fmt.Printf("  %v\n", err)
		}
		// Optionally: return fmt.Errorf("some peers failed to join the network")
	}

	// Join all orderers to the network
	ordererResults, ordererErrs := client.JoinAllOrderersToFabricNetwork(networkResp.ID, ordererNodeIDs)
	for _, resp := range ordererResults {
		fmt.Printf("Orderer joined network %d successfully. Network ID: %d, Status: %s\n", networkResp.ID, resp.ID, resp.Status)
	}
	if len(ordererErrs) > 0 {
		fmt.Println("Errors occurred while joining some orderers:")
		for _, err := range ordererErrs {
			fmt.Printf("  %v\n", err)
		}
		// Optionally: return fmt.Errorf("some orderers failed to join the network")
	}

	return nil
}

func NewFabricTestnetCmd() *cobra.Command {
	runner := &FabricTestnetRunner{
		Config: FabricTestnetConfig{},
	}

	cmd := &cobra.Command{
		Use:   "fabric",
		Short: "Create a Fabric testnet",
		Run: func(cmd *cobra.Command, args []string) {

			if err := runner.Run(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&runner.Config.Name, "name", "", "Name of the testnet (required)")
	cmd.Flags().IntVar(&runner.Config.Nodes, "nodes", 1, "Number of nodes (default 1)")
	cmd.Flags().StringVar(&runner.Config.Org, "org", "", "Organization MSP ID (required)")
	cmd.Flags().StringSliceVar(&runner.Config.PeerOrgs, "peerOrgs", nil, "List of peer organizations (comma-separated)")
	cmd.Flags().StringSliceVar(&runner.Config.OrdererOrgs, "ordererOrgs", nil, "List of orderer organizations (comma-separated)")
	cmd.Flags().StringSliceVar(&runner.Config.Channels, "channels", nil, "List of channels to create (comma-separated)")
	cmd.Flags().StringToIntVar(&runner.Config.PeerCounts, "peerCounts", nil, "Number of peers per org (e.g., Org1=2,Org2=3)")
	cmd.Flags().StringToIntVar(&runner.Config.OrdererCounts, "ordererCounts", nil, "Number of orderers per org (e.g., Orderer1=1,Orderer2=2)")
	cmd.Flags().StringVar(&runner.Config.Mode, "mode", "service", "Node mode (default 'service')")
	cmd.Flags().StringVar(&runner.Config.ExternalIP, "external-ip", "", "External IP address to use for node endpoints (defaults to auto-detection in docker mode)")

	return cmd
}
