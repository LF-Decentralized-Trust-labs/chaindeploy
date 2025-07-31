package fabric

import (
	"fmt"

	"github.com/chainlaunch/chainlaunch/cmd/common"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/spf13/cobra"
	// for NewListCmd
)

// NewFabricCmd returns the fabric command
func NewFabricCmd(logger *logger.Logger) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "fabric",
		Short: "Manage Fabric networks",
		Long:  `Create, update, and manage Hyperledger Fabric networks.`,
	}

	rootCmd.AddCommand(
		newCreateCmd(logger),
		newUpdateCmd(logger),
		newJoinCmd(logger),
		newJoinAllCmd(logger),
		newJoinOrdererCmd(logger),
		newJoinAllOrderersCmd(logger),
		NewListCmd(logger),
	)

	return rootCmd
}

func newJoinCmd(logger *logger.Logger) *cobra.Command {
	var networkID int64
	var peerID int64

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join a peer to a Fabric network",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := common.NewClientFromEnv()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			resp, err := client.JoinPeerToFabricNetwork(networkID, peerID)
			if err != nil {
				return fmt.Errorf("failed to join peer %d to network %d: %w", peerID, networkID, err)
			}
			fmt.Printf("Peer %d joined network %d successfully.\n", peerID, networkID)
			fmt.Printf("Network ID: %d\n", resp.ID)
			fmt.Printf("Status: %s\n", resp.Status)
			return nil
		},
	}
	cmd.Flags().Int64Var(&networkID, "network-id", 0, "Fabric network ID")
	cmd.Flags().Int64Var(&peerID, "peer-id", 0, "Peer node ID")
	cmd.MarkFlagRequired("network-id")
	cmd.MarkFlagRequired("peer-id")
	return cmd
}

func newJoinAllCmd(logger *logger.Logger) *cobra.Command {
	var networkID int64

	cmd := &cobra.Command{
		Use:   "join-all",
		Short: "Join all peers to a Fabric network",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := common.NewClientFromEnv()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			peerResp, err := client.ListPeerNodes(1, 1000)
			if err != nil {
				return fmt.Errorf("failed to list peer nodes: %w", err)
			}
			peerNodeIDs := []int64{}
			for _, peer := range peerResp.Items {
				peerNodeIDs = append(peerNodeIDs, peer.ID)
			}
			results, errs := client.JoinAllPeersToFabricNetwork(networkID, peerNodeIDs)
			for _, resp := range results {
				fmt.Printf("Peer joined network %d successfully. Network ID: %d, Status: %s\n", networkID, resp.ID, resp.Status)
			}
			if len(errs) > 0 {
				fmt.Println("Errors occurred while joining some peers:")
				for _, err := range errs {
					fmt.Printf("  %v\n", err)
				}
				return fmt.Errorf("some peers failed to join the network")
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&networkID, "network-id", 0, "Fabric network ID")
	cmd.MarkFlagRequired("network-id")
	return cmd
}

func newJoinOrdererCmd(logger *logger.Logger) *cobra.Command {
	var networkID int64
	var ordererID int64

	cmd := &cobra.Command{
		Use:   "join-orderer",
		Short: "Join an orderer to a Fabric network",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := common.NewClientFromEnv()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			resp, err := client.JoinOrdererToFabricNetwork(networkID, ordererID)
			if err != nil {
				return fmt.Errorf("failed to join orderer %d to network %d: %w", ordererID, networkID, err)
			}
			fmt.Printf("Orderer %d joined network %d successfully.\n", ordererID, networkID)
			fmt.Printf("Network ID: %d\n", resp.ID)
			fmt.Printf("Status: %s\n", resp.Status)
			return nil
		},
	}
	cmd.Flags().Int64Var(&networkID, "network-id", 0, "Fabric network ID")
	cmd.Flags().Int64Var(&ordererID, "orderer-id", 0, "Orderer node ID")
	cmd.MarkFlagRequired("network-id")
	cmd.MarkFlagRequired("orderer-id")
	return cmd
}

func newJoinAllOrderersCmd(logger *logger.Logger) *cobra.Command {
	var networkID int64

	cmd := &cobra.Command{
		Use:   "join-all-orderers",
		Short: "Join all orderers to a Fabric network",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := common.NewClientFromEnv()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			ordererResp, err := client.ListOrdererNodes(1, 1000)
			if err != nil {
				return fmt.Errorf("failed to list orderer nodes: %w", err)
			}
			ordererNodeIDs := []int64{}
			for _, orderer := range ordererResp.Items {
				ordererNodeIDs = append(ordererNodeIDs, orderer.ID)
			}
			results, errs := client.JoinAllOrderersToFabricNetwork(networkID, ordererNodeIDs)

			for _, resp := range results {
				fmt.Printf("Orderer joined network %d successfully. Network ID: %d, Status: %s\n", networkID, resp.ID, resp.Status)
			}
			if len(errs) > 0 {
				fmt.Println("Errors occurred while joining some orderers:")
				for _, err := range errs {
					fmt.Printf("  %v\n", err)
				}
				return fmt.Errorf("some orderers failed to join the network")
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&networkID, "network-id", 0, "Fabric network ID")
	cmd.MarkFlagRequired("network-id")
	return cmd
}
