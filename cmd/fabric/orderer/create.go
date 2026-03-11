package orderer

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chainlaunch/chainlaunch/cmd/common"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/spf13/cobra"
)

type createCmd struct {
	name           string
	mspID          string
	orgID          int64
	listenAddr     string
	adminAddr      string
	operationsAddr string
	externalAddr   string
	version        string
	domainNames    []string
	envVars        []string
	mode           string
	logger         *logger.Logger
}

func (c *createCmd) validate() error {
	if c.name == "" {
		return fmt.Errorf("name is required")
	}
	if c.mspID == "" {
		return fmt.Errorf("MSP ID is required")
	}
	if c.orgID == 0 {
		return fmt.Errorf("organization ID is required")
	}
	return nil
}

func (c *createCmd) parseEnvVars() (map[string]string, error) {
	env := make(map[string]string)
	for _, envVar := range c.envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid environment variable format: %s", envVar)
		}
		env[parts[0]] = parts[1]
	}
	return env, nil
}

func (c *createCmd) run(out io.Writer) error {
	client, err := common.NewClientFromEnv()
	if err != nil {
		return err
	}

	// Parse environment variables
	env, err := c.parseEnvVars()
	if err != nil {
		return err
	}

	// Prepare request
	req := &types.FabricOrdererConfig{
		BaseNodeConfig: types.BaseNodeConfig{
			Mode: c.mode,
		},
		Name:                    c.name,
		MSPID:                   c.mspID,
		OrganizationID:          c.orgID,
		ListenAddress:           c.listenAddr,
		AdminAddress:            c.adminAddr,
		OperationsListenAddress: c.operationsAddr,
		ExternalEndpoint:        c.externalAddr,
		Version:                 c.version,
		DomainNames:             c.domainNames,
		Env:                     env,
	}

	// Create orderer node
	node, err := client.CreateOrdererNode(req)
	if err != nil {
		return fmt.Errorf("failed to create orderer node: %w", err)
	}

	fmt.Fprintf(out, "Orderer node %s created successfully with ID %d\n", node.Name, node.ID)
	return nil
}

// NewCreateCmd returns the create orderer command
func NewCreateCmd(logger *logger.Logger) *cobra.Command {
	c := &createCmd{
		logger: logger,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new orderer node",
		Long:  `Create a new Hyperledger Fabric orderer node`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.validate(); err != nil {
				return err
			}
			return c.run(os.Stdout)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&c.name, "name", "n", "", "Name of the orderer node")
	flags.StringVarP(&c.mspID, "msp-id", "m", "", "MSP ID of the organization")
	flags.Int64VarP(&c.orgID, "org-id", "o", 0, "Organization ID")
	flags.StringVar(&c.listenAddr, "listen-addr", "0.0.0.0:7050", "Listen address for the orderer")
	flags.StringVar(&c.adminAddr, "admin-addr", "0.0.0.0:7053", "Admin listen address")
	flags.StringVar(&c.operationsAddr, "operations-addr", "0.0.0.0:9443", "Operations listen address")
	flags.StringVar(&c.externalAddr, "external-addr", "", "External endpoint for the orderer")
	flags.StringVar(&c.version, "version", "2.5.0", "Fabric version to use")
	flags.StringArrayVar(&c.domainNames, "domain", []string{}, "Domain names for the orderer")
	flags.StringArrayVar(&c.envVars, "env", []string{}, "Environment variables in KEY=VALUE format")
	flags.StringVar(&c.mode, "mode", "service", "Node mode: service or docker")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("msp-id")
	cmd.MarkFlagRequired("org-id")

	return cmd
}
