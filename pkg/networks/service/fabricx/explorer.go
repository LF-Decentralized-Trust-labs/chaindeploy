package fabricx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabricx/explorer/decoder"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/hyperledger/fabric-x-common/api/committerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ChainInfo summarizes ledger height, taken from the committer sidecar's
// BlockQueryService.GetBlockchainInfo.
type ChainInfo struct {
	Height            uint64 `json:"height"`
	CurrentBlockHash  string `json:"currentBlockHash,omitempty"`
	PreviousBlockHash string `json:"previousBlockHash,omitempty"`
}

// BlockSummary is the shape returned by the block list endpoint.
type BlockSummary struct {
	Number       uint64 `json:"number"`
	DataHash     string `json:"dataHash,omitempty"`
	PreviousHash string `json:"previousHash,omitempty"`
	TxCount      int    `json:"txCount"`
}

// NamespacePolicy is the shape returned by /namespace-policies.
type NamespacePolicy struct {
	Namespace string `json:"namespace"`
	Version   uint64 `json:"version"`
	Scheme    string `json:"scheme,omitempty"`
}

// StateRow is a single key/value hit against QueryService.GetRows.
type StateRow struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version uint64 `json:"version"`
}

// explorerClients holds live gRPC clients to one committer's sidecar and
// query-service. dial() returns one; callers must close via cleanup().
type explorerClients struct {
	Blocks  committerpb.BlockQueryServiceClient
	Query   committerpb.QueryServiceClient
	cleanup func()
}

// dialExplorer opens insecure gRPC connections to one committer in the network.
// It picks the first committer with a deployment config. The sidecar and
// query-service both run plaintext gRPC inside the committer group.
func (d *FabricXDeployer) dialExplorer(ctx context.Context, networkID int64) (*explorerClients, error) {
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network %d: %w", networkID, err)
	}
	if network.Platform != string(types.NetworkTypeFabricX) && network.Platform != "FABRICX" {
		return nil, fmt.Errorf("network %d is not a FabricX network", networkID)
	}
	if !network.Config.Valid {
		return nil, fmt.Errorf("network %d has no config", networkID)
	}
	var cfg types.FabricXNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse network config: %w", err)
	}

	var chosenGroupID, chosenNodeID int64
	for _, o := range cfg.Organizations {
		if o.CommitterNodeGroupID != 0 {
			chosenGroupID = o.CommitterNodeGroupID
			break
		}
		if o.CommitterNodeID != 0 {
			chosenNodeID = o.CommitterNodeID
			break
		}
	}
	if chosenGroupID == 0 && chosenNodeID == 0 {
		return nil, fmt.Errorf("no committer node found in network %d", networkID)
	}

	var deployCfg nodetypes.FabricXCommitterDeploymentConfig
	if chosenGroupID != 0 {
		grp, err := d.db.GetNodeGroup(ctx, chosenGroupID)
		if err != nil {
			return nil, fmt.Errorf("get committer node_group %d: %w", chosenGroupID, err)
		}
		if !grp.DeploymentConfig.Valid {
			return nil, fmt.Errorf("committer node_group %d has no deployment config", chosenGroupID)
		}
		if err := json.Unmarshal([]byte(grp.DeploymentConfig.String), &deployCfg); err != nil {
			return nil, fmt.Errorf("failed to parse committer group deployment config: %w", err)
		}
	} else {
		dbNode, err := d.db.GetNode(ctx, chosenNodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get committer %d: %w", chosenNodeID, err)
		}
		if !dbNode.DeploymentConfig.Valid {
			return nil, fmt.Errorf("committer %d has no deployment config", chosenNodeID)
		}
		if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &deployCfg); err != nil {
			return nil, fmt.Errorf("failed to parse committer deployment config: %w", err)
		}
	}

	host := deployCfg.ExternalIP
	if host == "" {
		host = "localhost"
	}
	if resolveLocalDev(&cfg, d.configService) {
		host = "127.0.0.1"
	}

	sidecarAddr := fmt.Sprintf("%s:%d", host, deployCfg.SidecarPort)
	queryAddr := fmt.Sprintf("%s:%d", host, deployCfg.QueryServicePort)

	sidecarConn, err := grpc.NewClient(sidecarAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial sidecar %s: %w", sidecarAddr, err)
	}
	queryConn, err := grpc.NewClient(queryAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		sidecarConn.Close()
		return nil, fmt.Errorf("dial query-service %s: %w", queryAddr, err)
	}

	return &explorerClients{
		Blocks: committerpb.NewBlockQueryServiceClient(sidecarConn),
		Query:  committerpb.NewQueryServiceClient(queryConn),
		cleanup: func() {
			sidecarConn.Close()
			queryConn.Close()
		},
	}, nil
}

// GetChainInfo returns ledger height from the committer sidecar.
func (d *FabricXDeployer) GetChainInfo(ctx context.Context, networkID int64) (*ChainInfo, error) {
	clients, err := d.dialExplorer(ctx, networkID)
	if err != nil {
		return nil, err
	}
	defer clients.cleanup()
	info, err := clients.Blocks.GetBlockchainInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("GetBlockchainInfo: %w", err)
	}
	return &ChainInfo{Height: info.GetHeight()}, nil
}

// GetBlocks returns a page of recent blocks. Latest first when reverse=true.
// Since the sidecar has no list endpoint we query GetBlockByNumber in a loop.
func (d *FabricXDeployer) GetBlocks(ctx context.Context, networkID int64, limit, offset int32, reverse bool) ([]BlockSummary, uint64, error) {
	clients, err := d.dialExplorer(ctx, networkID)
	if err != nil {
		return nil, 0, err
	}
	defer clients.cleanup()

	info, err := clients.Blocks.GetBlockchainInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, 0, fmt.Errorf("GetBlockchainInfo: %w", err)
	}
	height := info.GetHeight()
	if height == 0 {
		return nil, 0, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// Determine [lo, hi] block numbers to return.
	// height = chain length; highest block number = height - 1.
	var numbers []uint64
	if reverse {
		start := int64(height) - 1 - int64(offset)
		for i := int32(0); i < limit && start >= 0; i++ {
			numbers = append(numbers, uint64(start))
			start--
		}
	} else {
		start := uint64(offset)
		for i := int32(0); i < limit && start < height; i++ {
			numbers = append(numbers, start)
			start++
		}
	}

	out := make([]BlockSummary, 0, len(numbers))
	for _, n := range numbers {
		block, err := clients.Blocks.GetBlockByNumber(ctx, &committerpb.BlockNumber{Number: n})
		if err != nil {
			// Skip blocks the sidecar can't resolve; log-worthy but not fatal.
			continue
		}
		decoded, err := decoder.DecodeBlock(block)
		if err != nil {
			continue
		}
		out = append(out, BlockSummary{
			Number:       decoded.Number,
			DataHash:     decoded.DataHash,
			PreviousHash: decoded.PreviousHash,
			TxCount:      decoded.TxCount,
		})
	}
	return out, height, nil
}

// GetBlock returns a fully decoded block (with txs).
func (d *FabricXDeployer) GetBlock(ctx context.Context, networkID int64, blockNum uint64) (*decoder.DecodedBlock, error) {
	clients, err := d.dialExplorer(ctx, networkID)
	if err != nil {
		return nil, err
	}
	defer clients.cleanup()
	block, err := clients.Blocks.GetBlockByNumber(ctx, &committerpb.BlockNumber{Number: blockNum})
	if err != nil {
		return nil, fmt.Errorf("GetBlockByNumber %d: %w", blockNum, err)
	}
	return decoder.DecodeBlock(block)
}

// GetTransaction returns a decoded envelope by txID.
func (d *FabricXDeployer) GetTransaction(ctx context.Context, networkID int64, txID string) (*decoder.DecodedTx, error) {
	clients, err := d.dialExplorer(ctx, networkID)
	if err != nil {
		return nil, err
	}
	defer clients.cleanup()
	env, err := clients.Blocks.GetTxByID(ctx, &committerpb.TxID{TxId: txID})
	if err != nil {
		return nil, fmt.Errorf("GetTxByID %s: %w", txID, err)
	}
	return decoder.DecodeEnvelope(env)
}

// GetNamespacePolicies returns the list of channel namespaces from the
// query-service.
func (d *FabricXDeployer) GetNamespacePolicies(ctx context.Context, networkID int64) ([]NamespacePolicy, error) {
	clients, err := d.dialExplorer(ctx, networkID)
	if err != nil {
		return nil, err
	}
	defer clients.cleanup()
	resp, err := clients.Query.GetNamespacePolicies(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("GetNamespacePolicies: %w", err)
	}
	out := make([]NamespacePolicy, 0, len(resp.GetPolicies()))
	for _, p := range resp.GetPolicies() {
		out = append(out, NamespacePolicy{
			Namespace: p.GetNamespace(),
			Version:   p.GetVersion(),
		})
	}
	return out, nil
}

// GetNamespaceState returns key/value rows. If keys is empty this does
// nothing since FabricX's QueryService does not expose a full scan.
func (d *FabricXDeployer) GetNamespaceState(ctx context.Context, networkID int64, namespace string, keys []string) ([]StateRow, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	clients, err := d.dialExplorer(ctx, networkID)
	if err != nil {
		return nil, err
	}
	defer clients.cleanup()

	keyBytes := make([][]byte, 0, len(keys))
	for _, k := range keys {
		keyBytes = append(keyBytes, []byte(k))
	}
	resp, err := clients.Query.GetRows(ctx, &committerpb.Query{
		Namespaces: []*committerpb.QueryNamespace{{NsId: namespace, Keys: keyBytes}},
	})
	if err != nil {
		return nil, fmt.Errorf("GetRows: %w", err)
	}
	var out []StateRow
	for _, rns := range resp.GetNamespaces() {
		for _, row := range rns.GetRows() {
			out = append(out, StateRow{
				Key:     string(row.GetKey()),
				Value:   truncateBytes(row.GetValue(), 256),
				Version: row.GetVersion(),
			})
		}
	}
	return out, nil
}

func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
