package service

import (
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	orgservicefabric "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/besu"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabric"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabricx"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
)

// DeployerFactory creates network deployers based on blockchain platform
type DeployerFactory struct {
	db            *db.Queries
	nodes         *nodeservice.NodeService
	keyMgmt       *keymanagement.KeyManagementService
	orgService    *orgservicefabric.OrganizationService
	configService *config.ConfigService
}

// NewDeployerFactory creates a new deployer factory
func NewDeployerFactory(db *db.Queries, nodes *nodeservice.NodeService, keyMgmt *keymanagement.KeyManagementService, orgService *orgservicefabric.OrganizationService, configService *config.ConfigService) *DeployerFactory {
	return &DeployerFactory{
		db:            db,
		nodes:         nodes,
		keyMgmt:       keyMgmt,
		orgService:    orgService,
		configService: configService,
	}
}

// GetDeployer returns a deployer for the specified blockchain platform
func (f *DeployerFactory) GetDeployer(platform string) (types.NetworkDeployer, error) {
	switch platform {
	case string(BlockchainTypeFabric):
		return fabric.NewFabricDeployer(f.db, f.nodes, f.keyMgmt, f.orgService), nil
	case string(BlockchainTypeBesu):
		return besu.NewBesuDeployer(f.db, f.nodes, f.keyMgmt), nil
	case string(BlockchainTypeFabricX):
		return fabricx.NewFabricXDeployer(f.db, f.nodes, f.keyMgmt, f.orgService, f.configService), nil
	default:
		return nil, fmt.Errorf("unsupported blockchain platform: %s", platform)
	}
}

// GetFabricXDeployer returns the concrete FabricX deployer, used for FabricX-
// specific operations (namespace creation) that aren't part of the generic
// NetworkDeployer interface.
func (f *DeployerFactory) GetFabricXDeployer() *fabricx.FabricXDeployer {
	return fabricx.NewFabricXDeployer(f.db, f.nodes, f.keyMgmt, f.orgService, f.configService)
}
