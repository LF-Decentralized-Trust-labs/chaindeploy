package node

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/chainlaunch/chainlaunch/cmd/common"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/spf13/cobra"
)

type listCmd struct {
	page   int
	limit  int
	output string // "tsv" or "json"
	logger *logger.Logger
}

func (c *listCmd) run(out *os.File) error {
	client, err := common.NewClientFromEnv()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	nodes, err := client.ListBesuNodes(c.page, c.limit)
	if err != nil {
		return fmt.Errorf("failed to list Besu nodes: %w", err)
	}

	switch c.output {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(nodes.Items); err != nil {
			return fmt.Errorf("failed to encode nodes as JSON: %w", err)
		}
		return nil
	case "tsv":
		// Create tab writer
		w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tName\tType\tStatus\tRPC\tMetrics\tP2P")
		fmt.Fprintln(w, "--\t----\t----\t------\t----\t----\t----")

		// Print nodes
		for _, node := range nodes.Items {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				node.ID,
				node.Name,
				node.NodeType,
				node.Status,
				fmt.Sprintf("%s:%d", node.BesuNode.RPCHost, node.BesuNode.RPCPort),
				fmt.Sprintf("%s:%d", node.BesuNode.MetricsHost, node.BesuNode.MetricsPort),
				fmt.Sprintf("%s:%d", node.BesuNode.P2PHost, node.BesuNode.P2PPort),
			)
		}

		w.Flush()

		// Print pagination info
		fmt.Printf("\nPage %d of %d (Total: %d)\n", nodes.Page, nodes.PageCount, nodes.Total)
		if nodes.HasNextPage {
			fmt.Println("Use --page to view more results")
		}
		return nil
	default:
		return fmt.Errorf("unsupported output type: %s (must be 'tsv' or 'json')", c.output)
	}
}

// NewListCmd returns the list Besu nodes command
func NewListCmd(logger *logger.Logger) *cobra.Command {
	c := &listCmd{
		page:   1,
		limit:  10,
		output: "tsv",
		logger: logger,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Besu nodes",
		Long:  `List all Besu nodes with pagination support`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.run(os.Stdout)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&c.page, "page", 1, "Page number")
	flags.IntVar(&c.limit, "limit", 10, "Number of items per page")
	flags.StringVar(&c.output, "output", "tsv", "Output type: tsv or json")

	return cmd
}
