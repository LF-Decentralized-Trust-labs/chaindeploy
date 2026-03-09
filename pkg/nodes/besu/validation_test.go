package besu

import (
	"strings"
	"testing"
)

func validStartBesuOpts() StartBesuOpts {
	return StartBesuOpts{
		ID:             "test-node-1",
		P2PHost:        "127.0.0.1",
		P2PPort:        "30303",
		RPCHost:        "127.0.0.1",
		RPCPort:        "8545",
		ConsensusType:  "qbft",
		NetworkID:      1234,
		ChainID:        1337,
		GenesisFile:    `{"config":{},"alloc":{},"difficulty":"0x1","gasLimit":"0x1000000"}`,
		NodePrivateKey: "a" + strings.Repeat("b", 62) + "c",
		MinerAddress:   "0x" + strings.Repeat("ab", 20),
		Version:        "24.3.0",
	}
}

// --- validatePort tests ---

func TestValidatePort_Valid(t *testing.T) {
	for _, port := range []string{"1", "80", "8545", "30303", "65535"} {
		if err := validatePort(port); err != nil {
			t.Errorf("validatePort(%q) should be valid, got: %v", port, err)
		}
	}
}

func TestValidatePort_Invalid(t *testing.T) {
	cases := []struct {
		port string
		desc string
	}{
		{"", "empty"},
		{"0", "zero"},
		{"-1", "negative"},
		{"65536", "too high"},
		{"abc", "non-numeric"},
	}
	for _, tc := range cases {
		if err := validatePort(tc.port); err == nil {
			t.Errorf("validatePort(%q) [%s] should fail", tc.port, tc.desc)
		}
	}
}

// --- validateHost tests ---

func TestValidateHost_Valid(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "0.0.0.0", "localhost", "192.168.1.1", "example.com", "node1.example.com"} {
		if err := validateHost(host); err != nil {
			t.Errorf("validateHost(%q) should be valid, got: %v", host, err)
		}
	}
}

func TestValidateHost_Invalid(t *testing.T) {
	for _, host := range []string{"", "inva!id", "host with space"} {
		if err := validateHost(host); err == nil {
			t.Errorf("validateHost(%q) should fail", host)
		}
	}
}

// --- validateID tests ---

func TestValidateID_Valid(t *testing.T) {
	for _, id := range []string{"node1", "my-node", "my_node", "My Node 1"} {
		if err := validateID(id); err != nil {
			t.Errorf("validateID(%q) should be valid, got: %v", id, err)
		}
	}
}

func TestValidateID_Invalid(t *testing.T) {
	cases := []struct {
		id   string
		desc string
	}{
		{"", "empty"},
		{strings.Repeat("a", 256), "too long"},
		{"node@1", "special char"},
	}
	for _, tc := range cases {
		if err := validateID(tc.id); err == nil {
			t.Errorf("validateID(%q) [%s] should fail", tc.id, tc.desc)
		}
	}
}

// --- validateVersion tests ---

func TestValidateVersion_Valid(t *testing.T) {
	for _, v := range []string{"24.3.0", "24.3", "24.1.0-RC1", "25.7.0"} {
		if err := validateVersion(v); err != nil {
			t.Errorf("validateVersion(%q) should be valid, got: %v", v, err)
		}
	}
}

func TestValidateVersion_Invalid(t *testing.T) {
	for _, v := range []string{"", "abc", "24", "v24.3.0"} {
		if err := validateVersion(v); err == nil {
			t.Errorf("validateVersion(%q) should fail", v)
		}
	}
}

// --- validateConsensusType tests ---

func TestValidateConsensusType_Valid(t *testing.T) {
	for _, ct := range []string{"QBFT", "qbft", "IBFT2", "ibft2", "CLIQUE", "ETHASH", "POW"} {
		if err := validateConsensusType(ct); err != nil {
			t.Errorf("validateConsensusType(%q) should be valid, got: %v", ct, err)
		}
	}
}

func TestValidateConsensusType_Invalid(t *testing.T) {
	for _, ct := range []string{"invalid", "PAXOS", ""} {
		if err := validateConsensusType(ct); err == nil {
			t.Errorf("validateConsensusType(%q) should fail", ct)
		}
	}
}

// --- validateChainID tests ---

func TestValidateChainID_Valid(t *testing.T) {
	for _, id := range []int64{1337, 2018, 12345, 999999} {
		if err := validateChainID(id); err != nil {
			t.Errorf("validateChainID(%d) should be valid, got: %v", id, err)
		}
	}
}

func TestValidateChainID_Invalid(t *testing.T) {
	cases := []struct {
		id   int64
		desc string
	}{
		{0, "zero"},
		{-1, "negative"},
		{1, "Ethereum Mainnet reserved"},
		{5, "Goerli reserved"},
		{137, "Polygon Mainnet reserved"},
	}
	for _, tc := range cases {
		if err := validateChainID(tc.id); err == nil {
			t.Errorf("validateChainID(%d) [%s] should fail", tc.id, tc.desc)
		}
	}
}

// --- validateNetworkID tests ---

func TestValidateNetworkID_Valid(t *testing.T) {
	if err := validateNetworkID(1); err != nil {
		t.Errorf("validateNetworkID(1) should be valid, got: %v", err)
	}
}

func TestValidateNetworkID_Invalid(t *testing.T) {
	for _, id := range []int64{0, -1, -100} {
		if err := validateNetworkID(id); err == nil {
			t.Errorf("validateNetworkID(%d) should fail", id)
		}
	}
}

// --- validateGenesisFile tests ---

func TestValidateGenesisFile_Valid(t *testing.T) {
	genesis := `{"config":{},"alloc":{},"difficulty":"0x1","gasLimit":"0x1000000"}`
	if err := validateGenesisFile(genesis); err != nil {
		t.Errorf("validateGenesisFile should be valid, got: %v", err)
	}
}

func TestValidateGenesisFile_Invalid(t *testing.T) {
	cases := []struct {
		genesis string
		desc    string
	}{
		{"", "empty"},
		{"not json", "invalid json"},
		{`{"config":{}}`, "missing required fields"},
		{`{"config":{},"alloc":{}}`, "missing difficulty and gasLimit"},
	}
	for _, tc := range cases {
		if err := validateGenesisFile(tc.genesis); err == nil {
			t.Errorf("validateGenesisFile [%s] should fail", tc.desc)
		}
	}
}

// --- validateNodePrivateKey tests ---

func TestValidateNodePrivateKey_Valid(t *testing.T) {
	key := strings.Repeat("ab", 32) // 64 hex chars
	if err := validateNodePrivateKey(key); err != nil {
		t.Errorf("validateNodePrivateKey should be valid, got: %v", err)
	}
	// With 0x prefix
	if err := validateNodePrivateKey("0x" + key); err != nil {
		t.Errorf("validateNodePrivateKey with 0x prefix should be valid, got: %v", err)
	}
}

func TestValidateNodePrivateKey_Invalid(t *testing.T) {
	cases := []struct {
		key  string
		desc string
	}{
		{"", "empty"},
		{"abc", "too short"},
		{strings.Repeat("g", 64), "non-hex"},
		{strings.Repeat("ab", 33), "too long"},
	}
	for _, tc := range cases {
		if err := validateNodePrivateKey(tc.key); err == nil {
			t.Errorf("validateNodePrivateKey [%s] should fail", tc.desc)
		}
	}
}

// --- validateEthereumAddress tests ---

func TestValidateEthereumAddress_Valid(t *testing.T) {
	addr := strings.Repeat("ab", 20) // 40 hex chars
	if err := validateEthereumAddress(addr); err != nil {
		t.Errorf("validateEthereumAddress should be valid, got: %v", err)
	}
	// With 0x prefix
	if err := validateEthereumAddress("0x" + addr); err != nil {
		t.Errorf("validateEthereumAddress with 0x prefix should be valid, got: %v", err)
	}
	// Empty is valid (optional)
	if err := validateEthereumAddress(""); err != nil {
		t.Errorf("validateEthereumAddress empty should be valid, got: %v", err)
	}
}

func TestValidateEthereumAddress_Invalid(t *testing.T) {
	cases := []struct {
		addr string
		desc string
	}{
		{"abc", "too short"},
		{strings.Repeat("g", 40), "non-hex"},
		{strings.Repeat("ab", 21), "too long"},
	}
	for _, tc := range cases {
		if err := validateEthereumAddress(tc.addr); err == nil {
			t.Errorf("validateEthereumAddress [%s] should fail", tc.desc)
		}
	}
}

// --- validateBootnode tests ---

func TestValidateBootnode_Valid(t *testing.T) {
	pubkey := strings.Repeat("ab", 64) // 128 hex chars
	enode := "enode://" + pubkey + "@192.168.1.1:30303"
	if err := validateBootnode(enode); err != nil {
		t.Errorf("validateBootnode should be valid, got: %v", err)
	}
	// With query params
	enode2 := enode + "?discport=0"
	if err := validateBootnode(enode2); err != nil {
		t.Errorf("validateBootnode with query should be valid, got: %v", err)
	}
}

func TestValidateBootnode_Invalid(t *testing.T) {
	cases := []struct {
		node string
		desc string
	}{
		{"", "empty"},
		{"http://example.com", "wrong scheme"},
		{"enode://short@1.2.3.4:30303", "short pubkey"},
	}
	for _, tc := range cases {
		if err := validateBootnode(tc.node); err == nil {
			t.Errorf("validateBootnode [%s] should fail", tc.desc)
		}
	}
}

// --- validateMetricsProtocol tests ---

func TestValidateMetricsProtocol_Valid(t *testing.T) {
	for _, p := range []string{"HTTP", "HTTPS", "PROMETHEUS", "http", "https", "prometheus"} {
		if err := validateMetricsProtocol(p); err != nil {
			t.Errorf("validateMetricsProtocol(%q) should be valid, got: %v", p, err)
		}
	}
}

func TestValidateMetricsProtocol_Invalid(t *testing.T) {
	for _, p := range []string{"", "TCP", "grpc"} {
		if err := validateMetricsProtocol(p); err == nil {
			t.Errorf("validateMetricsProtocol(%q) should fail", p)
		}
	}
}

// --- validateJWTAlgorithm tests ---

func TestValidateJWTAlgorithm_Valid(t *testing.T) {
	for _, algo := range []JWTAlgorithm{JWTAlgorithmRS256, JWTAlgorithmRS384, JWTAlgorithmRS512, JWTAlgorithmES256, JWTAlgorithmES384, JWTAlgorithmES512} {
		if err := validateJWTAlgorithm(algo); err != nil {
			t.Errorf("validateJWTAlgorithm(%q) should be valid, got: %v", algo, err)
		}
	}
}

func TestValidateJWTAlgorithm_Invalid(t *testing.T) {
	for _, algo := range []JWTAlgorithm{"", "HS256", "NONE"} {
		if err := validateJWTAlgorithm(algo); err == nil {
			t.Errorf("validateJWTAlgorithm(%q) should fail", algo)
		}
	}
}

// --- StartBesuOpts.Validate() integration tests ---

func TestValidate_ValidOpts(t *testing.T) {
	opts := validStartBesuOpts()
	if err := opts.Validate(); err != nil {
		t.Errorf("Validate() should pass with valid opts, got: %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	opts := validStartBesuOpts()
	opts.ID = ""
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with empty ID")
	}
	if !strings.Contains(err.Error(), "ID") {
		t.Errorf("error should mention ID, got: %v", err)
	}
}

func TestValidate_InvalidP2PPort(t *testing.T) {
	opts := validStartBesuOpts()
	opts.P2PPort = "99999"
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with invalid P2P port")
	}
	if !strings.Contains(err.Error(), "P2P port") {
		t.Errorf("error should mention P2P port, got: %v", err)
	}
}

func TestValidate_PortConflict(t *testing.T) {
	opts := validStartBesuOpts()
	opts.RPCPort = opts.P2PPort // Same port
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with port conflict")
	}
	if !strings.Contains(err.Error(), "port conflict") {
		t.Errorf("error should mention port conflict, got: %v", err)
	}
}

func TestValidate_MetricsPortConflict(t *testing.T) {
	opts := validStartBesuOpts()
	opts.MetricsEnabled = true
	opts.MetricsPort = 30303 // Same as P2P
	opts.MetricsProtocol = "PROMETHEUS"
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with metrics port conflict")
	}
	if !strings.Contains(err.Error(), "port conflict") {
		t.Errorf("error should mention port conflict, got: %v", err)
	}
}

func TestValidate_MetricsEnabled_ValidConfig(t *testing.T) {
	opts := validStartBesuOpts()
	opts.MetricsEnabled = true
	opts.MetricsPort = 9545
	opts.MetricsProtocol = "PROMETHEUS"
	if err := opts.Validate(); err != nil {
		t.Errorf("Validate() should pass with valid metrics config, got: %v", err)
	}
}

func TestValidate_MetricsEnabled_InvalidPort(t *testing.T) {
	opts := validStartBesuOpts()
	opts.MetricsEnabled = true
	opts.MetricsPort = 0
	opts.MetricsProtocol = "PROMETHEUS"
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with invalid metrics port")
	}
	if !strings.Contains(err.Error(), "metrics port") {
		t.Errorf("error should mention metrics port, got: %v", err)
	}
}

func TestValidate_MetricsEnabled_InvalidProtocol(t *testing.T) {
	opts := validStartBesuOpts()
	opts.MetricsEnabled = true
	opts.MetricsPort = 9545
	opts.MetricsProtocol = "INVALID"
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with invalid metrics protocol")
	}
	if !strings.Contains(err.Error(), "metrics protocol") {
		t.Errorf("error should mention metrics protocol, got: %v", err)
	}
}

func TestValidate_NegativeMinGasPrice(t *testing.T) {
	opts := validStartBesuOpts()
	opts.MinGasPrice = -1
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with negative min gas price")
	}
	if !strings.Contains(err.Error(), "min gas price") {
		t.Errorf("error should mention min gas price, got: %v", err)
	}
}

func TestValidate_JWTEnabled_MissingPublicKey(t *testing.T) {
	opts := validStartBesuOpts()
	opts.JWTEnabled = true
	opts.JWTPublicKeyContent = ""
	opts.JWTAuthenticationAlgorithm = JWTAlgorithmRS256
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with JWT enabled but no public key")
	}
	if !strings.Contains(err.Error(), "JWT public key") {
		t.Errorf("error should mention JWT public key, got: %v", err)
	}
}

func TestValidate_JWTEnabled_InvalidAlgorithm(t *testing.T) {
	opts := validStartBesuOpts()
	opts.JWTEnabled = true
	opts.JWTPublicKeyContent = "some-key-content"
	opts.JWTAuthenticationAlgorithm = "INVALID"
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with invalid JWT algorithm")
	}
	if !strings.Contains(err.Error(), "JWT algorithm") {
		t.Errorf("error should mention JWT algorithm, got: %v", err)
	}
}

func TestValidate_JWTEnabled_ValidConfig(t *testing.T) {
	opts := validStartBesuOpts()
	opts.JWTEnabled = true
	opts.JWTPublicKeyContent = "some-public-key-content"
	opts.JWTAuthenticationAlgorithm = JWTAlgorithmES256
	if err := opts.Validate(); err != nil {
		t.Errorf("Validate() should pass with valid JWT config, got: %v", err)
	}
}

func TestValidate_InvalidAccountsAllowList(t *testing.T) {
	opts := validStartBesuOpts()
	opts.AccountsAllowList = []string{"not-a-valid-address"}
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with invalid accounts allow list entry")
	}
	if !strings.Contains(err.Error(), "accountsAllowList") {
		t.Errorf("error should mention accountsAllowList, got: %v", err)
	}
}

func TestValidate_InvalidNodesAllowList(t *testing.T) {
	opts := validStartBesuOpts()
	opts.NodesAllowList = []string{"not-an-enode"}
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with invalid nodes allow list entry")
	}
	if !strings.Contains(err.Error(), "nodesAllowList") {
		t.Errorf("error should mention nodesAllowList, got: %v", err)
	}
}

func TestValidate_InvalidBootnodes(t *testing.T) {
	opts := validStartBesuOpts()
	opts.BootNodes = []string{"bad-enode"}
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with invalid bootnode")
	}
	if !strings.Contains(err.Error(), "bootNodes") {
		t.Errorf("error should mention bootNodes, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	opts := StartBesuOpts{} // All fields empty/zero
	err := opts.Validate()
	if err == nil {
		t.Fatal("Validate() should fail with empty opts")
	}
	// Should contain multiple error lines
	if !strings.Contains(err.Error(), "besu validation failed") {
		t.Errorf("error should contain 'besu validation failed', got: %v", err)
	}
	// Count number of error entries
	lines := strings.Split(err.Error(), "\n  - ")
	if len(lines) < 5 {
		t.Errorf("expected at least 5 validation errors, got %d in: %v", len(lines), err)
	}
}

// --- checkPortConflicts tests ---

func TestCheckPortConflicts_NoDuplicates(t *testing.T) {
	opts := validStartBesuOpts()
	if err := opts.checkPortConflicts(); err != nil {
		t.Errorf("checkPortConflicts should pass, got: %v", err)
	}
}

func TestCheckPortConflicts_P2PAndRPCSame(t *testing.T) {
	opts := validStartBesuOpts()
	opts.RPCPort = opts.P2PPort
	err := opts.checkPortConflicts()
	if err == nil {
		t.Fatal("checkPortConflicts should fail when P2P and RPC use same port")
	}
}

func TestCheckPortConflicts_MetricsConflict(t *testing.T) {
	opts := validStartBesuOpts()
	opts.MetricsEnabled = true
	opts.MetricsPort = 8545 // Same as RPC
	err := opts.checkPortConflicts()
	if err == nil {
		t.Fatal("checkPortConflicts should fail when metrics uses RPC port")
	}
}
