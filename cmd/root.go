/*
Copyright © 2025 ChainLaunch <dviejo@chainlaunch.dev>
*/
package cmd

import (
	"github.com/chainlaunch/chainlaunch/cmd/backup"
	"github.com/chainlaunch/chainlaunch/cmd/besu"
	"github.com/chainlaunch/chainlaunch/cmd/fabric"
	"github.com/chainlaunch/chainlaunch/cmd/fabricx"
	"github.com/chainlaunch/chainlaunch/cmd/keymanagement"
	"github.com/chainlaunch/chainlaunch/cmd/metrics"
	"github.com/chainlaunch/chainlaunch/cmd/networks"
	"github.com/chainlaunch/chainlaunch/cmd/serve"
	"github.com/chainlaunch/chainlaunch/cmd/testnet"
	"github.com/chainlaunch/chainlaunch/cmd/version"
	"github.com/chainlaunch/chainlaunch/config"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/spf13/cobra"
	// Add this import for the testnet command
	// import "lfdt-chainlaunch/cmd/testnet"
)

// rootCmd represents the base command when called without any subcommands

func NewRootCmd(configCMD config.ConfigCMD) *cobra.Command {
	logger := logger.NewDefault()
	rootCmd := &cobra.Command{
		Use:   "chainlaunch",
		Short: "A blockchain deployment API server",
		Long:  `chainlaunch is an API server for managing blockchain deployments.`,
	}

	rootCmd.AddCommand(serve.Command(configCMD, logger))
	rootCmd.AddCommand(fabric.NewFabricCmd(logger))
	rootCmd.AddCommand(fabricx.NewFabricXCmd(logger))
	rootCmd.AddCommand(version.NewVersionCmd())
	rootCmd.AddCommand(backup.NewBackupCmd())
	rootCmd.AddCommand(besu.NewBesuCmd(logger))
	rootCmd.AddCommand(networks.NewNetworksCmd(logger))
	rootCmd.AddCommand(keymanagement.NewKeyManagementCmd())
	rootCmd.AddCommand(testnet.NewTestnetCmd())
	rootCmd.AddCommand(metrics.NewMetricsCmd())
	// In the function where rootCmd is defined and commands are added:
	// rootCmd.AddCommand(testnet.NewTestnetCmd())
	return rootCmd
}
