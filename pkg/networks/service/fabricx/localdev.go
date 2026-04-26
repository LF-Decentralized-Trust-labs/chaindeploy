package fabricx

import (
	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
)

// resolveLocalDev returns whether a FabricX network should run in local-dev
// mode (Docker Desktop compatibility: host.docker.internal substitution and
// 127.0.0.1 rewriting for host-originated dials).
//
// Precedence:
//  1. The network's config.localDev field — per-network flag surfaced at
//     POST /networks/fabricx.
//  2. The CHAINLAUNCH_FABRICX_LOCAL_DEV env var via configService — global
//     fallback for existing installations that set the env var once.
//
// A nil configService is tolerated and counts as "fallback false".
func resolveLocalDev(networkConfig *types.FabricXNetworkConfig, cfg *config.ConfigService) bool {
	if networkConfig != nil && networkConfig.LocalDev {
		return true
	}
	if cfg != nil && cfg.FabricXLocalDev() {
		return true
	}
	return false
}
