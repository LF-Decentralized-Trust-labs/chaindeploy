package service

// Mode represents the deployment mode
type Mode string

const (
	ModeService Mode = "service"
	ModeDocker  Mode = "docker"
)

// NodeDefaults represents default values for a node
type NodeDefaults struct {
	ListenAddress           string `json:"listenAddress"`
	ExternalEndpoint        string `json:"externalEndpoint"`
	ChaincodeAddress        string `json:"chaincodeAddress,omitempty"`
	EventsAddress           string `json:"eventsAddress,omitempty"`
	OperationsListenAddress string `json:"operationsListenAddress"`
	AdminAddress            string `json:"adminAddress,omitempty"`
	Mode                    Mode   `json:"mode"`
	ContainerName           string `json:"containerName,omitempty"`
	ServiceName             string `json:"serviceName,omitempty"`
	LogPath                 string `json:"logPath,omitempty"`
	ErrorLogPath            string `json:"errorLogPath,omitempty"`
}

// BesuNodeDefaults represents default values for a Besu node
type BesuNodeDefaults struct {
	P2PHost    string            `json:"p2pHost"`
	P2PPort    uint              `json:"p2pPort"`
	RPCHost    string            `json:"rpcHost"`
	RPCPort    uint              `json:"rpcPort"`
	ExternalIP string            `json:"externalIp"`
	InternalIP string            `json:"internalIp"`
	Mode       Mode              `json:"mode"`
	Env        map[string]string `json:"environmentVariables"`
	// Metrics configuration
	MetricsEnabled  bool   `json:"metricsEnabled"`
	MetricsHost     string `json:"metricsHost"`
	MetricsPort     uint   `json:"metricsPort"`
	MetricsProtocol string `json:"metricsProtocol"`
}

// NodesDefaultsParams represents parameters for getting multiple nodes defaults
type NodesDefaultsParams struct {
	PeerCount    int  `json:"peerCount" validate:"min=0"`
	OrdererCount int  `json:"ordererCount" validate:"min=0"`
	Mode         Mode `json:"mode"`
}

// NodesDefaultsResult represents the result of getting multiple nodes defaults
type NodesDefaultsResult struct {
	Peers              []NodeDefaults `json:"peers"`
	Orderers           []NodeDefaults `json:"orderers"`
	AvailableAddresses []string       `json:"availableAddresses"`
}
