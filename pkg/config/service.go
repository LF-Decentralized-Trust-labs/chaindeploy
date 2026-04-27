package config

import "os"

// ConfigService handles configuration paths and directories
type ConfigService struct {
	dataPath string
}

// NewConfigService creates a new ConfigService instance
func NewConfigService(dataPath string) *ConfigService {
	return &ConfigService{
		dataPath: dataPath,
	}
}

func (s *ConfigService) GetDataPath() string {
	return s.dataPath
}

// FabricXLocalDev reports whether the FabricX deployer should assume a local
// Docker Desktop environment (macOS/Windows). In that mode, containers cannot
// reach the configured externalIP directly via hairpin NAT, so we inject an
// extra_hosts entry pointing the externalIP at host-gateway. Set via env var
// CHAINLAUNCH_FABRICX_LOCAL_DEV=true.
func (s *ConfigService) FabricXLocalDev() bool {
	v := os.Getenv("CHAINLAUNCH_FABRICX_LOCAL_DEV")
	return v == "1" || v == "true" || v == "TRUE" || v == "yes"
}
