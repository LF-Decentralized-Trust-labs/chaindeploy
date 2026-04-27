// Package fabricx holds CLI subcommands for FabricX (Arma consensus) networks.
// The web UI has a "FabricX Quick Start" button that provisions a 4-party
// network; this package exposes the same flow as a `chainlaunch fabricx
// quickstart` command so the same bundle can be spun up from the terminal, CI,
// or shell scripts without clicking through the browser.
package fabricx

import (
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/spf13/cobra"
)

// NewFabricXCmd returns the fabricx command group.
func NewFabricXCmd(log *logger.Logger) *cobra.Command {
	root := &cobra.Command{
		Use:   "fabricx",
		Short: "Manage FabricX (Arma consensus) networks",
		Long:  `FabricX is the high-throughput Hyperledger Fabric variant with Arma consensus.`,
	}
	root.AddCommand(newQuickstartCmd(log))
	return root
}
