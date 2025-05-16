package besu

// StartBesuOpts represents the options for starting a Besu node
type StartBesuOpts struct {
	ID             string            `json:"id"`
	ListenAddress  string            `json:"listenAddress"`
	P2PHost        string            `json:"p2pHost"`
	P2PPort        string            `json:"p2pPort"`
	RPCHost        string            `json:"rpcHost"`
	RPCPort        string            `json:"rpcPort"`
	ConsensusType  string            `json:"consensusType"`
	NetworkID      int64             `json:"networkId"`
	ChainID        int64             `json:"chainId"`
	GenesisFile    string            `json:"genesisFile"`
	NodePrivateKey string            `json:"nodePrivateKey"`
	MinerAddress   string            `json:"minerAddress"`
	BootNodes      []string          `json:"bootNodes"`
	Env            map[string]string `json:"env"`
	Version        string            `json:"version"`
	// Metrics configuration
	MetricsEnabled  bool   `json:"metricsEnabled"`
	MetricsPort     int64  `json:"metricsPort"`
	MetricsProtocol string `json:"metricsProtocol"`
}

// BesuConfig represents the configuration for a Besu node
type BesuConfig struct {
	Mode           string   `json:"mode"`
	ListenAddress  string   `json:"listenAddress"`
	P2PPort        string   `json:"p2pPort"`
	RPCPort        string   `json:"rpcPort"`
	ConsensusType  string   `json:"consensusType"`
	NetworkID      int64    `json:"networkId"`
	NodePrivateKey string   `json:"nodePrivateKey"`
	MinerAddress   string   `json:"minerAddress"`
	BootNodes      []string `json:"bootNodes"`
	DataDir        string   `json:"dataDir"`
}

// StartServiceResponse represents the response when starting a Besu node as a service
type StartServiceResponse struct {
	Mode        string `json:"mode"`
	Type        string `json:"type"`
	ServiceName string `json:"serviceName"`
}

// StartDockerResponse represents the response when starting a Besu node as a docker container
type StartDockerResponse struct {
	Mode          string `json:"mode"`
	ContainerName string `json:"containerName"`
}
