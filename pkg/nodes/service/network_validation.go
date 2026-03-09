package service

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/besu"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/orderer"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/peer"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/utils"
)

// PortUsage represents port usage information
type PortUsage struct {
	NodeID   int64
	NodeName string
	NodeType types.NodeType
	Port     string
	Purpose  string // e.g., "listen", "chaincode", "rpc", etc.
}

// NetworkLevelValidator provides network-level validation across all nodes
type NetworkLevelValidator struct {
	service *NodeService
}

// NewNetworkLevelValidator creates a new network-level validator
func NewNetworkLevelValidator(service *NodeService) *NetworkLevelValidator {
	return &NetworkLevelValidator{
		service: service,
	}
}

// ValidatePeerPorts validates that peer ports don't conflict with existing nodes
func (v *NetworkLevelValidator) ValidatePeerPorts(ctx context.Context, opts *peer.StartPeerOpts, excludeNodeID int64) error {
	// Extract ports from peer opts
	portsToCheck := map[string]string{
		"listen":     extractPort(opts.ListenAddress),
		"chaincode":  extractPort(opts.ChaincodeAddress),
		"events":     extractPort(opts.EventsAddress),
		"operations": extractPort(opts.OperationsListenAddress),
	}

	// Get existing port usage
	existingPorts, err := v.getAllPortUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing port usage: %w", err)
	}

	// Check for conflicts
	var conflicts []string
	for purpose, port := range portsToCheck {
		if port == "" {
			continue
		}

		for _, usage := range existingPorts {
			// Skip if this is the same node being updated
			if usage.NodeID == excludeNodeID {
				continue
			}

			if usage.Port == port {
				conflicts = append(conflicts, fmt.Sprintf(
					"peer %s port %s conflicts with %s node '%s' (%s port)",
					purpose, port, usage.NodeType, usage.NodeName, usage.Purpose,
				))
			}
		}
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("port conflicts detected:\n  - %s", strings.Join(conflicts, "\n  - "))
	}

	return nil
}

// ValidateOrdererPorts validates that orderer ports don't conflict with existing nodes
func (v *NetworkLevelValidator) ValidateOrdererPorts(ctx context.Context, opts *orderer.StartOrdererOpts, excludeNodeID int64) error {
	// Extract ports from orderer opts
	portsToCheck := map[string]string{
		"listen":     extractPort(opts.ListenAddress),
		"admin":      extractPort(opts.AdminListenAddress),
		"operations": extractPort(opts.OperationsListenAddress),
	}

	// Get existing port usage
	existingPorts, err := v.getAllPortUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing port usage: %w", err)
	}

	// Check for conflicts
	var conflicts []string
	for purpose, port := range portsToCheck {
		if port == "" {
			continue
		}

		for _, usage := range existingPorts {
			// Skip if this is the same node being updated
			if usage.NodeID == excludeNodeID {
				continue
			}

			if usage.Port == port {
				conflicts = append(conflicts, fmt.Sprintf(
					"orderer %s port %s conflicts with %s node '%s' (%s port)",
					purpose, port, usage.NodeType, usage.NodeName, usage.Purpose,
				))
			}
		}
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("port conflicts detected:\n  - %s", strings.Join(conflicts, "\n  - "))
	}

	return nil
}

// ValidateBesuPorts validates that Besu ports don't conflict with existing nodes
func (v *NetworkLevelValidator) ValidateBesuPorts(ctx context.Context, opts *besu.StartBesuOpts, excludeNodeID int64) error {
	// Extract ports from Besu opts
	portsToCheck := map[string]string{
		"p2p": opts.P2PPort,
		"rpc": opts.RPCPort,
	}

	if opts.MetricsEnabled {
		portsToCheck["metrics"] = fmt.Sprintf("%d", opts.MetricsPort)
	}

	// Get existing port usage
	existingPorts, err := v.getAllPortUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing port usage: %w", err)
	}

	// Check for conflicts
	var conflicts []string
	for purpose, port := range portsToCheck {
		if port == "" {
			continue
		}

		for _, usage := range existingPorts {
			// Skip if this is the same node being updated
			if usage.NodeID == excludeNodeID {
				continue
			}

			if usage.Port == port {
				conflicts = append(conflicts, fmt.Sprintf(
					"besu %s port %s conflicts with %s node '%s' (%s port)",
					purpose, port, usage.NodeType, usage.NodeName, usage.Purpose,
				))
			}
		}
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("port conflicts detected:\n  - %s", strings.Join(conflicts, "\n  - "))
	}

	return nil
}

// getAllPortUsage gets all ports currently in use by nodes
func (v *NetworkLevelValidator) getAllPortUsage(ctx context.Context) ([]PortUsage, error) {
	// Get all nodes (use a high limit to get all nodes, no pagination needed for validation)
	nodes, err := v.service.db.ListNodes(ctx, &db.ListNodesParams{
		Limit:  10000, // High limit to get all nodes
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var portUsage []PortUsage

	for _, node := range nodes {
		// Load node config
		nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
		if err != nil {
			v.service.logger.Warn("Failed to load config for node during port validation",
				"nodeID", node.ID,
				"error", err)
			continue
		}

		nodeType := types.NodeType(node.NodeType.String)

		switch nodeType {
		case types.NodeTypeFabricPeer:
			peerConfig, ok := nodeConfig.(*types.FabricPeerConfig)
			if ok {
				portUsage = append(portUsage, extractPeerPorts(node.ID, node.Name, peerConfig)...)
			}

		case types.NodeTypeFabricOrderer:
			ordererConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
			if ok {
				portUsage = append(portUsage, extractOrdererPorts(node.ID, node.Name, ordererConfig)...)
			}

		case types.NodeTypeBesuFullnode:
			besuConfig, ok := nodeConfig.(*types.BesuNodeConfig)
			if ok {
				portUsage = append(portUsage, extractBesuPorts(node.ID, node.Name, besuConfig)...)
			}
		}
	}

	return portUsage, nil
}

// extractPeerPorts extracts all ports from a peer configuration
func extractPeerPorts(nodeID int64, nodeName string, config *types.FabricPeerConfig) []PortUsage {
	var ports []PortUsage

	if port := extractPort(config.ListenAddress); port != "" {
		ports = append(ports, PortUsage{
			NodeID:   nodeID,
			NodeName: nodeName,
			NodeType: types.NodeTypeFabricPeer,
			Port:     port,
			Purpose:  "listen",
		})
	}

	if port := extractPort(config.ChaincodeAddress); port != "" {
		ports = append(ports, PortUsage{
			NodeID:   nodeID,
			NodeName: nodeName,
			NodeType: types.NodeTypeFabricPeer,
			Port:     port,
			Purpose:  "chaincode",
		})
	}

	if port := extractPort(config.EventsAddress); port != "" {
		ports = append(ports, PortUsage{
			NodeID:   nodeID,
			NodeName: nodeName,
			NodeType: types.NodeTypeFabricPeer,
			Port:     port,
			Purpose:  "events",
		})
	}

	if port := extractPort(config.OperationsListenAddress); port != "" {
		ports = append(ports, PortUsage{
			NodeID:   nodeID,
			NodeName: nodeName,
			NodeType: types.NodeTypeFabricPeer,
			Port:     port,
			Purpose:  "operations",
		})
	}

	return ports
}

// extractOrdererPorts extracts all ports from an orderer configuration
func extractOrdererPorts(nodeID int64, nodeName string, config *types.FabricOrdererConfig) []PortUsage {
	var ports []PortUsage

	if port := extractPort(config.ListenAddress); port != "" {
		ports = append(ports, PortUsage{
			NodeID:   nodeID,
			NodeName: nodeName,
			NodeType: types.NodeTypeFabricOrderer,
			Port:     port,
			Purpose:  "listen",
		})
	}

	if port := extractPort(config.AdminAddress); port != "" {
		ports = append(ports, PortUsage{
			NodeID:   nodeID,
			NodeName: nodeName,
			NodeType: types.NodeTypeFabricOrderer,
			Port:     port,
			Purpose:  "admin",
		})
	}

	if port := extractPort(config.OperationsListenAddress); port != "" {
		ports = append(ports, PortUsage{
			NodeID:   nodeID,
			NodeName: nodeName,
			NodeType: types.NodeTypeFabricOrderer,
			Port:     port,
			Purpose:  "operations",
		})
	}

	return ports
}

// extractBesuPorts extracts all ports from a Besu configuration
func extractBesuPorts(nodeID int64, nodeName string, config *types.BesuNodeConfig) []PortUsage {
	var ports []PortUsage

	ports = append(ports, PortUsage{
		NodeID:   nodeID,
		NodeName: nodeName,
		NodeType: types.NodeTypeBesuFullnode,
		Port:     fmt.Sprintf("%d", config.P2PPort),
		Purpose:  "p2p",
	})

	ports = append(ports, PortUsage{
		NodeID:   nodeID,
		NodeName: nodeName,
		NodeType: types.NodeTypeBesuFullnode,
		Port:     fmt.Sprintf("%d", config.RPCPort),
		Purpose:  "rpc",
	})

	if config.MetricsEnabled {
		ports = append(ports, PortUsage{
			NodeID:   nodeID,
			NodeName: nodeName,
			NodeType: types.NodeTypeBesuFullnode,
			Port:     fmt.Sprintf("%d", config.MetricsPort),
			Purpose:  "metrics",
		})
	}

	return ports
}

// extractPort extracts the port number from an address string (host:port)
func extractPort(address string) string {
	if address == "" {
		return ""
	}

	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return ""
	}

	return port
}

// GetPortUsageReport generates a human-readable port usage report
func (v *NetworkLevelValidator) GetPortUsageReport(ctx context.Context) (string, error) {
	portUsage, err := v.getAllPortUsage(ctx)
	if err != nil {
		return "", err
	}

	if len(portUsage) == 0 {
		return "No ports currently in use by nodes", nil
	}

	var report strings.Builder
	report.WriteString("Port Usage Report:\n")
	report.WriteString("=================\n\n")

	// Group by port
	portMap := make(map[string][]PortUsage)
	for _, usage := range portUsage {
		portMap[usage.Port] = append(portMap[usage.Port], usage)
	}

	// Check for conflicts
	hasConflicts := false
	for port, usages := range portMap {
		if len(usages) > 1 {
			hasConflicts = true
			report.WriteString(fmt.Sprintf("PORT %s - CONFLICT (used by %d nodes):\n", port, len(usages)))
		} else {
			report.WriteString(fmt.Sprintf("PORT %s:\n", port))
		}

		for _, usage := range usages {
			report.WriteString(fmt.Sprintf("   - %s '%s' (%s)\n", usage.NodeType, usage.NodeName, usage.Purpose))
		}
		report.WriteString("\n")
	}

	if hasConflicts {
		report.WriteString("\nWARNING: Port conflicts detected! Some nodes may fail to start.\n")
	}

	return report.String(), nil
}
