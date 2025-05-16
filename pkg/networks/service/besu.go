package service

import (
	"context"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/networks/service/besu"
)

func (s *NetworkService) importBesuNetwork(ctx context.Context, params ImportNetworkParams) (*ImportNetworkResult, error) {
	// Get the Besu deployer
	deployer, err := s.deployerFactory.GetDeployer("besu")
	if err != nil {
		return nil, fmt.Errorf("failed to get Besu deployer: %w", err)
	}

	besuDeployer, ok := deployer.(*besu.BesuDeployer)
	if !ok {
		return nil, fmt.Errorf("invalid deployer type")
	}

	// Import the network using the Besu deployer
	networkID, err := besuDeployer.ImportNetwork(ctx, params.GenesisFile, params.Name, params.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to import Besu network: %w", err)
	}

	return &ImportNetworkResult{
		NetworkID: networkID,
		Message:   "Besu network imported successfully",
	}, nil
}
