package xsource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	ptypes "github.com/chainlaunch/chainlaunch/pkg/plugin/types"
)

// FabricPeerValue represents a fabric-peer x-source value
type FabricPeerValue struct {
	BaseXSourceValue
	PeerIDs     []string
	queries     *db.Queries
	nodeService *nodeservice.NodeService
}

// NewFabricPeerHandler creates a new handler for fabric-peer x-source type
func NewFabricPeerHandler(queries *db.Queries, nodeService *nodeservice.NodeService) XSourceHandler {
	return &fabricPeerHandler{
		queries:     queries,
		nodeService: nodeService,
	}
}

type fabricPeerHandler struct {
	queries     *db.Queries
	nodeService *nodeservice.NodeService
}

func (h *fabricPeerHandler) GetType() XSourceType {
	return FabricPeer
}

func (h *fabricPeerHandler) CreateValue(key string, rawValue interface{}) (XSourceValue, error) {
	var peerIDs []string

	switch v := rawValue.(type) {
	case string:
		peerIDs = []string{v}
	case float64:
		peerIDs = []string{strconv.FormatInt(int64(v), 10)}
	case int:
		peerIDs = []string{strconv.FormatInt(int64(v), 10)}
	case []interface{}:
		peerIDs = make([]string, len(v))
		for i, item := range v {
			switch val := item.(type) {
			case string:
				peerIDs[i] = val
			case float64:
				peerIDs[i] = strconv.FormatInt(int64(val), 10)
			case int:
				peerIDs[i] = strconv.FormatInt(int64(val), 10)
			default:
				return nil, fmt.Errorf("fabric-peer array elements must be strings or numbers")
			}
		}
	default:
		return nil, fmt.Errorf("fabric-peer value must be a string, number, or array of strings/numbers")
	}

	return &FabricPeerValue{
		BaseXSourceValue: BaseXSourceValue{RawValue: rawValue, Key: key},
		PeerIDs:          peerIDs,
		queries:          h.queries,
		nodeService:      h.nodeService,
	}, nil
}

func (h *fabricPeerHandler) ListOptions(ctx context.Context) ([]OptionItem, error) {
	value := &FabricPeerValue{
		queries:     h.queries,
		nodeService: h.nodeService,
	}
	return value.ListOptions(ctx)
}

func (v *FabricPeerValue) ListOptions(ctx context.Context) ([]OptionItem, error) {
	// Get all Fabric peers from the node service
	platform := types.PlatformFabric
	peers, err := v.nodeService.ListNodes(ctx, &platform, 1, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to list peers: %w", err)
	}

	var opts []OptionItem
	for _, peer := range peers.Items {
		if peer.NodeType == types.NodeTypeFabricPeer {
			opts = append(opts, OptionItem{
				Label: fmt.Sprintf("%s (%s)", peer.Name, peer.FabricPeer.MSPID),
				Value: fmt.Sprintf("%d", peer.ID),
			})
		}
	}
	return opts, nil
}

func (v *FabricPeerValue) Validate(ctx context.Context) error {
	options, err := v.ListOptions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list fabric peer options: %w", err)
	}

	// Create a map of valid peer IDs for faster lookup
	validPeers := make(map[string]bool)
	for _, opt := range options {
		validPeers[opt.Value] = true
	}

	// Validate each peer ID
	for _, peerID := range v.PeerIDs {
		if !validPeers[peerID] {
			return fmt.Errorf("invalid fabric peer ID: %s", peerID)
		}
	}

	return nil
}

func (v *FabricPeerValue) GetValue(ctx context.Context, spec ptypes.ParameterSpec) (interface{}, error) {
	var details []*FabricPeerDetails

	for _, peerIDStr := range v.PeerIDs {
		peerID, err := strconv.ParseInt(peerIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid peer ID format: %w", err)
		}

		peer, err := v.nodeService.GetNode(ctx, peerID)
		if err != nil {
			return nil, fmt.Errorf("failed to get peer: %w", err)
		}

		if peer.FabricPeer == nil {
			return nil, fmt.Errorf("peer is not a Fabric peer")
		}

		// Define the TLS cert path inside the container
		tlsCertPath := fmt.Sprintf("/etc/chainlaunch/peers/%d/tls/cert.pem", peerID)
		tlsCACertPath := fmt.Sprintf("/etc/chainlaunch/peers/%d/tls/ca.pem", peerID)
		details = append(details, &FabricPeerDetails{
			ID:               peerID,
			Name:             peer.Name,
			ExternalEndpoint: peer.FabricPeer.ExternalEndpoint,
			TLSCert:          peer.FabricPeer.TLSCert,
			MspID:            peer.FabricPeer.MSPID,
			OrgID:            peer.FabricPeer.OrganizationID,
			TLSCertPath:      tlsCertPath,
			TLSCACertPath:    tlsCACertPath,
		})
	}

	// Return array if spec type is array, otherwise return single peer if available
	if spec.Type == "array" {
		return details, nil
	}
	if len(details) == 1 {
		return details[0], nil
	}
	return details, nil
}

// GetVolumeMounts returns the volume mounts needed for this peer
func (v *FabricPeerValue) GetVolumeMounts(ctx context.Context) ([]VolumeMount, error) {
	var mounts []VolumeMount

	for _, peerIDStr := range v.PeerIDs {
		peerID, err := strconv.ParseInt(peerIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid peer ID format: %w", err)
		}

		peer, err := v.nodeService.GetNode(ctx, peerID)
		if err != nil {
			return nil, fmt.Errorf("failed to get peer: %w", err)
		}

		if peer.FabricPeer == nil {
			return nil, fmt.Errorf("peer is not a Fabric peer")
		}

		// Create a temporary file for the TLS cert
		tempDir := fmt.Sprintf("/tmp/chainlaunch/peers/%d/tls", peerID)
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}

		certPath := filepath.Join(tempDir, "cert.pem")
		if err := os.WriteFile(certPath, []byte(peer.FabricPeer.TLSCert), 0644); err != nil {
			return nil, fmt.Errorf("failed to write TLS cert: %w", err)
		}

		caCertPath := filepath.Join(tempDir, "ca.pem")
		if err := os.WriteFile(caCertPath, []byte(peer.FabricPeer.TLSCACert), 0644); err != nil {
			return nil, fmt.Errorf("failed to write TLS CA cert: %w", err)
		}

		// Add volume mount for the TLS cert
		mounts = append(mounts, VolumeMount{
			Source:      certPath,
			Target:      fmt.Sprintf("/etc/chainlaunch/peers/%d/tls/cert.pem", peerID),
			Type:        "bind",
			ReadOnly:    true,
			Description: fmt.Sprintf("TLS certificate for peer %s", peer.Name),
		})
		mounts = append(mounts, VolumeMount{
			Source:      caCertPath,
			Target:      fmt.Sprintf("/etc/chainlaunch/peers/%d/tls/ca.pem", peerID),
			Type:        "bind",
			ReadOnly:    true,
			Description: fmt.Sprintf("TLS CA certificate for peer %s", peer.Name),
		})
	}

	return mounts, nil
}

// FabricPeerDetails represents the details of a Fabric peer
type FabricPeerDetails struct {
	ID               int64
	Name             string
	ExternalEndpoint string
	TLSCert          string
	MspID            string
	OrgID            int64
	TLSCertPath      string // Path inside the container
	TLSCACertPath    string // Path inside the container
}
