package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabricx"
)

// JoinFabricXNodeToNetwork is the stage-2 entry point of the FabricX two-stage
// deployment: it fetches the (already-created) network's genesis block, writes
// it into the node's config directories, and starts the node.
//
// Stage 1 happens when the orderer group / committer is created via
// POST /nodes — certs and config are generated (Init), but the container is
// NOT started.
//
// Stage 2 (this method) is called after CreateNetwork has produced the genesis
// block from the party configs. It:
//  1. loads the genesis block
//  2. calls the deployer's JoinNode which writes the genesis and starts the
//     node via NodeService.StartNode.
func (s *NetworkService) JoinFabricXNodeToNetwork(ctx context.Context, networkID, nodeID int64) error {
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}
	if network.Platform != string(BlockchainTypeFabricX) {
		return fmt.Errorf("network %d is not a FabricX network (platform=%s)", networkID, network.Platform)
	}
	if !network.GenesisBlockB64.Valid {
		return fmt.Errorf("genesis block is not set for network %d", networkID)
	}

	genesisBlock, err := base64.StdEncoding.DecodeString(network.GenesisBlockB64.String)
	if err != nil {
		return fmt.Errorf("failed to decode genesis block: %w", err)
	}

	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return fmt.Errorf("failed to get deployer: %w", err)
	}

	if err := deployer.JoinNode(networkID, genesisBlock, nodeID); err != nil {
		return fmt.Errorf("failed to join FabricX node %d to network %d: %w", nodeID, networkID, err)
	}

	// Once every network_node row for this network is in the "joined"
	// state, kick off the verification smoke test. This is best-effort —
	// the join itself is what the caller is waiting for; verification
	// updates the network status asynchronously and surfaces errors via
	// the structured logs (and via /networks/{id} status).
	if allJoined, err := s.allFabricXNodesJoined(ctx, networkID); err == nil && allJoined {
		go func() {
			verifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if vErr := s.deployerFactory.GetFabricXDeployer().VerifyNetwork(verifyCtx, networkID); vErr != nil {
				// Already logged inside VerifyNetwork; nothing to do
				// here besides not propagating to the join caller.
				_ = vErr
			}
		}()
	}
	return nil
}

// allFabricXNodesJoined returns true if every network_node row for the
// given network is in a state that means the underlying node has the
// genesis block on disk. We treat anything other than the literal
// "pending" status as "joined" — the JoinNode path flips rows to
// "joined" before container start, and a transient docker-mount error
// during start does not invalidate that.
func (s *NetworkService) allFabricXNodesJoined(ctx context.Context, networkID int64) (bool, error) {
	rows, err := s.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	for _, r := range rows {
		if r.Status == "pending" {
			return false, nil
		}
	}
	return true, nil
}

// VerifyFabricXNetwork is the operator-facing entry point for the
// post-provisioning smoke test. It is also called automatically by
// JoinFabricXNodeToNetwork once every node has joined; the explicit
// endpoint exists so the operator can re-run verification after fixing
// a problem (e.g. a flaky orderer) without having to recreate any
// nodes.
func (s *NetworkService) VerifyFabricXNetwork(ctx context.Context, networkID int64) error {
	return s.deployerFactory.GetFabricXDeployer().VerifyNetwork(ctx, networkID)
}

// CreateFabricXNamespace broadcasts a namespace-creation tx to the network's
// orderer router, signed by the given submitter organization, and records the
// attempt in the database.
func (s *NetworkService) CreateFabricXNamespace(ctx context.Context, networkID int64, opts fabricx.NamespaceCreateOptions) (*fabricx.NamespaceResult, error) {
	return s.deployerFactory.GetFabricXDeployer().CreateNamespace(ctx, networkID, opts)
}

// ListFabricXNamespaces returns all namespace records for a network.
func (s *NetworkService) ListFabricXNamespaces(ctx context.Context, networkID int64) ([]*db.FabricxNamespace, error) {
	return s.deployerFactory.GetFabricXDeployer().ListNamespaces(ctx, networkID)
}

// ListFabricXNamespacesMerged returns the merged chain+DB view. The second
// return value is a non-fatal "chain unreachable" error; DB rows are still
// returned in that case.
func (s *NetworkService) ListFabricXNamespacesMerged(ctx context.Context, networkID int64) ([]fabricx.NamespaceView, error, error) {
	return s.deployerFactory.GetFabricXDeployer().ListNamespacesMerged(ctx, networkID)
}

// DeleteFabricXNamespaceRecord removes the local record only (on-chain is
// update-only and cannot be deleted).
func (s *NetworkService) DeleteFabricXNamespaceRecord(ctx context.Context, id int64) error {
	return s.deployerFactory.GetFabricXDeployer().DeleteNamespaceRecord(ctx, id)
}

// GetFabricXChainInfo returns the ledger height for a FabricX network.
func (s *NetworkService) GetFabricXChainInfo(ctx context.Context, networkID int64) (*fabricx.ChainInfo, error) {
	return s.deployerFactory.GetFabricXDeployer().GetChainInfo(ctx, networkID)
}

// GetFabricXBlocks returns a paginated list of blocks for a FabricX network.
func (s *NetworkService) GetFabricXBlocks(ctx context.Context, networkID int64, limit, offset int32, reverse bool) ([]fabricx.BlockSummary, uint64, error) {
	return s.deployerFactory.GetFabricXDeployer().GetBlocks(ctx, networkID, limit, offset, reverse)
}

// GetFabricXBlock returns a decoded block for a FabricX network.
func (s *NetworkService) GetFabricXBlock(ctx context.Context, networkID int64, blockNum uint64) (any, error) {
	return s.deployerFactory.GetFabricXDeployer().GetBlock(ctx, networkID, blockNum)
}

// GetFabricXTransaction returns a decoded envelope by txID.
func (s *NetworkService) GetFabricXTransaction(ctx context.Context, networkID int64, txID string) (any, error) {
	return s.deployerFactory.GetFabricXDeployer().GetTransaction(ctx, networkID, txID)
}

// GetFabricXNamespacePolicies returns the list of channel namespaces from the
// committer query-service.
func (s *NetworkService) GetFabricXNamespacePolicies(ctx context.Context, networkID int64) ([]fabricx.NamespacePolicy, error) {
	return s.deployerFactory.GetFabricXDeployer().GetNamespacePolicies(ctx, networkID)
}

// GetFabricXNamespaceState queries state rows for specific keys within a
// namespace. FabricX does not expose a full scan.
func (s *NetworkService) GetFabricXNamespaceState(ctx context.Context, networkID int64, namespace string, keys []string) ([]fabricx.StateRow, error) {
	return s.deployerFactory.GetFabricXDeployer().GetNamespaceState(ctx, networkID, namespace, keys)
}

// PublishFabricXPublicParams broadcasts the token-sdk ZK public parameters
// into the named namespace on a FabricX channel, signed by the submitter org's
// admin key. No DB row is written — PPs are not a first-class DB entity.
func (s *NetworkService) PublishFabricXPublicParams(ctx context.Context, networkID int64, opts fabricx.PublishPublicParamsOptions) (*fabricx.PublishPublicParamsResult, error) {
	return s.deployerFactory.GetFabricXDeployer().PublishPublicParams(ctx, networkID, opts)
}
