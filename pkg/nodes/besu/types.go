package besu

// JWTAlgorithm represents the supported JWT authentication algorithms
type JWTAlgorithm string

const (
	JWTAlgorithmRS256 JWTAlgorithm = "RS256"
	JWTAlgorithmRS384 JWTAlgorithm = "RS384"
	JWTAlgorithmRS512 JWTAlgorithm = "RS512"
	JWTAlgorithmES256 JWTAlgorithm = "ES256"
	JWTAlgorithmES384 JWTAlgorithm = "ES384"
	JWTAlgorithmES512 JWTAlgorithm = "ES512"
)

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
	// Gas and access control configuration
	MinGasPrice   int64  `json:"minGasPrice"`
	HostAllowList string `json:"hostAllowList"`
	// Permissions configuration
	AccountsAllowList []string `json:"accountsAllowList"`
	NodesAllowList    []string `json:"nodesAllowList"`
	// JWT Authentication configuration
	JWTEnabled                 bool         `json:"jwtEnabled"`
	JWTPublicKeyContent        string       `json:"jwtPublicKeyContent"`
	JWTAuthenticationAlgorithm JWTAlgorithm `json:"jwtAuthenticationAlgorithm"`
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
