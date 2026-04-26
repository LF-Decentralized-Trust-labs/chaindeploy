package utils

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// LoadNodeConfig deserializes a stored config based on its type
func LoadNodeConfig(data []byte) (types.NodeConfig, error) {
	return LoadNodeConfigWithHint(data, "")
}

// LoadNodeConfigWithHint deserializes a stored node config. If the stored
// envelope (or inner config) type is empty — as occurs on some legacy
// FabricX rows — the hint (derived from e.g. the nodes.node_type column)
// is used to select the concrete config struct.
func LoadNodeConfigWithHint(data []byte, hint string) (types.NodeConfig, error) {
	var stored types.StoredConfig
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stored config: %w", err)
	}

	// Legacy rows may have been written with an empty outer envelope type.
	// Fall back to reading the type embedded in the inner config payload.
	if stored.Type == "" && len(stored.Config) > 0 {
		var inner struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(stored.Config, &inner); err == nil && inner.Type != "" {
			stored.Type = inner.Type
		}
	}

	// Final fallback: use the caller-supplied hint (e.g. nodes.node_type).
	if stored.Type == "" {
		switch hint {
		case "FABRIC_PEER":
			stored.Type = "fabric-peer"
		case "FABRIC_ORDERER":
			stored.Type = "fabric-orderer"
		case "BESU_FULLNODE":
			stored.Type = "besu"
		case "FABRICX_ORDERER_GROUP":
			stored.Type = "fabricx-orderer-group"
		case "FABRICX_COMMITTER":
			stored.Type = "fabricx-committer"
		case "FABRICX_ORDERER_ROUTER",
			"FABRICX_ORDERER_BATCHER",
			"FABRICX_ORDERER_CONSENTER",
			"FABRICX_ORDERER_ASSEMBLER",
			"FABRICX_COMMITTER_SIDECAR",
			"FABRICX_COMMITTER_COORDINATOR",
			"FABRICX_COMMITTER_VALIDATOR",
			"FABRICX_COMMITTER_VERIFIER",
			"FABRICX_COMMITTER_QUERY_SERVICE":
			stored.Type = "fabricx-child"
		}
	}

	switch stored.Type {
	case "fabric-peer":
		var config types.FabricPeerConfig
		logrus.Debug("stored.Config", "stored.Config", string(stored.Config))
		if err := json.Unmarshal(stored.Config, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabric peer config: %w", err)
		}
		logrus.Debug("config", "config", config)
		return &config, nil

	case "fabric-orderer":
		var config types.FabricOrdererConfig
		if err := json.Unmarshal(stored.Config, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabric orderer config: %w", err)
		}
		return &config, nil

	case "besu":
		var config types.BesuNodeConfig
		if err := json.Unmarshal(stored.Config, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal besu config: %w", err)
		}
		return &config, nil

	case "fabricx-orderer-group":
		var config types.FabricXOrdererGroupConfig
		if err := json.Unmarshal(stored.Config, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabricx orderer group config: %w", err)
		}
		return &config, nil

	case "fabricx-committer":
		var config types.FabricXCommitterConfig
		if err := json.Unmarshal(stored.Config, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabricx committer config: %w", err)
		}
		return &config, nil

	case "fabricx-child":
		var config types.FabricXChildConfig
		if err := json.Unmarshal(stored.Config, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabricx child config: %w", err)
		}
		return &config, nil

	default:
		return nil, fmt.Errorf("unsupported node type: %s", stored.Type)
	}
}

// Add this helper function to deserialize deployment config
func DeserializeDeploymentConfig(configJSON string) (types.NodeDeploymentConfig, error) {
	// First unmarshal to get the type
	var baseConfig struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(configJSON), &baseConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal base config: %w", err)
	}

	// Based on the type, unmarshal into the appropriate struct
	var config types.NodeDeploymentConfig
	switch baseConfig.Type {
	case "fabric-peer":
		var c types.FabricPeerDeploymentConfig
		if err := json.Unmarshal([]byte(configJSON), &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabric peer config: %w", err)
		}
		config = &c
	case "fabric-orderer":
		var c types.FabricOrdererDeploymentConfig
		if err := json.Unmarshal([]byte(configJSON), &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabric orderer config: %w", err)
		}
		config = &c
	case "besu":
		var c types.BesuNodeDeploymentConfig
		if err := json.Unmarshal([]byte(configJSON), &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal besu config: %w", err)
		}
		config = &c
	case "fabricx-orderer-group":
		var c types.FabricXOrdererGroupDeploymentConfig
		if err := json.Unmarshal([]byte(configJSON), &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabricx orderer group config: %w", err)
		}
		config = &c
	case "fabricx-committer":
		var c types.FabricXCommitterDeploymentConfig
		if err := json.Unmarshal([]byte(configJSON), &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabricx committer config: %w", err)
		}
		config = &c
	case "fabricx-child":
		var c types.FabricXChildDeploymentConfig
		if err := json.Unmarshal([]byte(configJSON), &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabricx child config: %w", err)
		}
		config = &c
	default:
		return nil, fmt.Errorf("unknown node type: %s", baseConfig.Type)
	}

	return config, nil
}

// StoreNodeConfig serializes a node config with its type information
func StoreNodeConfig(config types.NodeConfig) ([]byte, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	stored := types.StoredConfig{
		Type:   config.GetType(),
		Config: configBytes,
	}

	return json.Marshal(stored)
}
