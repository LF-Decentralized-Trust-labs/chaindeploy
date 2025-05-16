package networkconfig

// NetworkConfig represents the root structure of the network configuration
type NetworkConfig struct {
	Name                   string                          `yaml:"name"`
	Version                string                          `yaml:"version"`
	Client                 ClientConfig                    `yaml:"client"`
	Organizations          map[string]Organization         `yaml:"organizations"`
	Orderers               map[string]Orderer              `yaml:"orderers"`
	Peers                  map[string]Peer                 `yaml:"peers"`
	CertificateAuthorities map[string]CertificateAuthority `yaml:"certificateAuthorities"`
	Channels               map[string]Channel              `yaml:"channels"`
}

// ClientConfig represents the client configuration
type ClientConfig struct {
	Organization string `yaml:"organization"`
}

// Organization represents an organization in the network
type Organization struct {
	MSPID      string          `yaml:"mspid"`
	CryptoPath string          `yaml:"cryptoPath"`
	Users      map[string]User `yaml:"users"`
	Peers      []string        `yaml:"peers"`
	Orderers   []string        `yaml:"orderers"`
}

// User represents a user in an organization
type User struct {
	Cert UserCert `yaml:"cert"`
	Key  UserKey  `yaml:"key"`
}

// UserCert represents a user's certificate
type UserCert struct {
	PEM  string `yaml:"pem,omitempty"`
	Path string `yaml:"path,omitempty"`
}

// UserKey represents a user's private key
type UserKey struct {
	PEM  string `yaml:"pem,omitempty"`
	Path string `yaml:"path,omitempty"`
}

// Orderer represents an orderer node
type Orderer struct {
	URL          string      `yaml:"url"`
	AdminURL     string      `yaml:"adminUrl"`
	AdminTLSCert string      `yaml:"adminTlsCert"`
	GRPCOptions  GRPCOptions `yaml:"grpcOptions"`
	TLSCACerts   TLSCACerts  `yaml:"tlsCACerts"`
}

// Peer represents a peer node
type Peer struct {
	URL         string      `yaml:"url"`
	GRPCOptions GRPCOptions `yaml:"grpcOptions"`
	TLSCACerts  TLSCACerts  `yaml:"tlsCACerts"`
}

// GRPCOptions represents gRPC options
type GRPCOptions struct {
	AllowInsecure bool `yaml:"allow-insecure"`
}

// TLSCACerts represents TLS CA certificates
type TLSCACerts struct {
	PEM  string `yaml:"pem,omitempty"`
	Path string `yaml:"path,omitempty"`
}

// CertificateAuthority represents a CA server
type CertificateAuthority struct {
	URL        string     `yaml:"url"`
	Registrar  Registrar  `yaml:"registrar"`
	CAName     string     `yaml:"caName"`
	TLSCACerts TLSCACerts `yaml:"tlsCACerts"`
}

// Registrar represents CA registrar information
type Registrar struct {
	EnrollID     string `yaml:"enrollId"`
	EnrollSecret string `yaml:"enrollSecret"`
}

// Channel represents a channel configuration
type Channel struct {
	Orderers []string              `yaml:"orderers"`
	Peers    map[string]PeerConfig `yaml:"peers"`
}

// PeerConfig represents peer configuration within a channel
type PeerConfig struct {
	Discover       bool `yaml:"discover"`
	EndorsingPeer  bool `yaml:"endorsingPeer"`
	ChaincodeQuery bool `yaml:"chaincodeQuery"`
	LedgerQuery    bool `yaml:"ledgerQuery"`
	EventSource    bool `yaml:"eventSource"`
}
