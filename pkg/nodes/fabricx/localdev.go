package fabricx

import (
	"context"
	"encoding/json"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
)

// fabricXNetworkConfigProbe is a tiny partial copy of
// networks/service/types.FabricXNetworkConfig. Duplicated here (instead of
// importing the types package) to avoid a nodes → networks/service import
// cycle. Only the one field we need is unmarshaled.
type fabricXNetworkConfigProbe struct {
	LocalDev bool `json:"localDev,omitempty"`
}

// resolveLocalDevForNode returns whether the FabricX node should be run in
// local-dev mode (container extra_hosts, postgres host rewrites, etc.).
//
// Precedence:
//  1. The node's network's config.localDev flag. Looked up via network_nodes
//     → networks. If the node is associated with multiple networks (shouldn't
//     happen for FabricX today) the first one's flag wins.
//  2. The CHAINLAUNCH_FABRICX_LOCAL_DEV env var via configService, as a
//     global fallback for existing installations.
//
// Errors from the DB are swallowed and fall back to the env var — we must
// never block node start on this lookup.
func resolveLocalDevForNode(ctx context.Context, q *db.Queries, cfg *config.ConfigService, nodeID int64) bool {
	if q != nil && nodeID > 0 {
		rows, err := q.ListNetworkNodesByNode(ctx, nodeID)
		if err == nil && len(rows) > 0 {
			for _, nn := range rows {
				net, err := q.GetNetwork(ctx, nn.NetworkID)
				if err != nil {
					continue
				}
				if !net.Config.Valid || net.Config.String == "" {
					continue
				}
				var probe fabricXNetworkConfigProbe
				if err := json.Unmarshal([]byte(net.Config.String), &probe); err != nil {
					continue
				}
				if probe.LocalDev {
					return true
				}
			}
		}
	}
	if cfg != nil && cfg.FabricXLocalDev() {
		return true
	}
	return false
}
