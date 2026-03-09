package besu

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// validatePort checks if a port string is valid
func validatePort(port string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	// Check if it's a valid number between 1 and 65535
	var portNum int
	_, err := fmt.Sscanf(port, "%d", &portNum)
	if err != nil {
		return fmt.Errorf("invalid port format: %w", err)
	}
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", portNum)
	}
	return nil
}

// validateHost validates a host address (IP or domain name)
func validateHost(host string) error {
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// Check if it's a valid IP address
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}

	// Check if it's a valid domain name or special values like "0.0.0.0"
	if host == "0.0.0.0" || host == "localhost" {
		return nil
	}

	// Domain name validation
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	if !domainRegex.MatchString(host) {
		return fmt.Errorf("invalid host: %s", host)
	}

	return nil
}

// validateID validates a node ID
func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("node ID cannot be empty")
	}

	// Check for reasonable length
	if len(id) > 255 {
		return fmt.Errorf("node ID too long (max 255 characters)")
	}

	// ID should contain only alphanumeric, hyphens, underscores, and spaces
	idRegex := regexp.MustCompile(`^[a-zA-Z0-9\-_ ]+$`)
	if !idRegex.MatchString(id) {
		return fmt.Errorf("node ID can only contain letters, numbers, hyphens, underscores, and spaces")
	}

	return nil
}

// validateVersion validates a Besu version string
func validateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Besu version format check (24.3.0, 24.3.0-RC1, etc.)
	versionRegex := regexp.MustCompile(`^\d+\.\d+(\.\d+)?(-[a-zA-Z0-9]+)?$`)
	if !versionRegex.MatchString(version) {
		return fmt.Errorf("invalid version format: %s (expected format: 24.3.0 or 24.3.0-RC1)", version)
	}

	return nil
}

// validateConsensusType validates the consensus mechanism type
func validateConsensusType(consensusType string) error {
	validTypes := map[string]bool{
		"QBFT":   true,
		"IBFT2":  true,
		"CLIQUE": true,
		"ETHASH": true,
		"POW":    true,
		"qbft":   true,
		"ibft2":  true,
		"clique": true,
		"ethash": true,
		"pow":    true,
	}

	if !validTypes[consensusType] {
		return fmt.Errorf("invalid consensus type: %s (valid types: QBFT, IBFT2, CLIQUE, ETHASH)", consensusType)
	}

	return nil
}

// validateChainID validates the chain ID
func validateChainID(chainID int64) error {
	if chainID <= 0 {
		return fmt.Errorf("chain ID must be positive, got %d", chainID)
	}

	// Warn about common public chain IDs to avoid confusion
	publicChainIDs := map[int64]string{
		1:        "Ethereum Mainnet",
		3:        "Ropsten Testnet",
		4:        "Rinkeby Testnet",
		5:        "Goerli Testnet",
		11155111: "Sepolia Testnet",
		137:      "Polygon Mainnet",
		80001:    "Polygon Mumbai Testnet",
	}

	if name, exists := publicChainIDs[chainID]; exists {
		return fmt.Errorf("chain ID %d is reserved for %s. Please use a different chain ID for private networks", chainID, name)
	}

	return nil
}

// validateNetworkID validates the network ID
func validateNetworkID(networkID int64) error {
	if networkID <= 0 {
		return fmt.Errorf("network ID must be positive, got %d", networkID)
	}
	return nil
}

// validateGenesisFile validates the genesis file content
func validateGenesisFile(genesisFile string) error {
	if genesisFile == "" {
		return fmt.Errorf("genesis file content cannot be empty")
	}

	// Try to parse as JSON to ensure it's valid
	var genesis map[string]interface{}
	if err := json.Unmarshal([]byte(genesisFile), &genesis); err != nil {
		return fmt.Errorf("invalid genesis file JSON: %w", err)
	}

	// Check for required fields
	requiredFields := []string{"config", "alloc", "difficulty", "gasLimit"}
	for _, field := range requiredFields {
		if _, exists := genesis[field]; !exists {
			return fmt.Errorf("genesis file missing required field: %s", field)
		}
	}

	return nil
}

// validateNodePrivateKey validates the node private key format
func validateNodePrivateKey(privateKey string) error {
	if privateKey == "" {
		return fmt.Errorf("node private key cannot be empty")
	}

	// Remove 0x prefix if present
	privateKey = strings.TrimPrefix(privateKey, "0x")

	// Check if it's a valid hex string of correct length (64 characters for 32 bytes)
	if len(privateKey) != 64 {
		return fmt.Errorf("node private key must be 64 hex characters (32 bytes)")
	}

	hexRegex := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	if !hexRegex.MatchString(privateKey) {
		return fmt.Errorf("node private key must be a valid hexadecimal string")
	}

	return nil
}

// validateEthereumAddress validates an Ethereum address
func validateEthereumAddress(address string) error {
	if address == "" {
		return nil // Miner address is optional
	}

	// Remove 0x prefix if present
	address = strings.TrimPrefix(address, "0x")

	// Check if it's a valid hex string of correct length (40 characters for 20 bytes)
	if len(address) != 40 {
		return fmt.Errorf("ethereum address must be 40 hex characters (20 bytes)")
	}

	hexRegex := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	if !hexRegex.MatchString(address) {
		return fmt.Errorf("ethereum address must be a valid hexadecimal string")
	}

	return nil
}

// validateBootnode validates a bootnode enode URL
func validateBootnode(bootnode string) error {
	if bootnode == "" {
		return fmt.Errorf("bootnode cannot be empty")
	}

	// Enode URL format: enode://pubkey@ip:port or enode://pubkey@ip:port?discport=0
	enodeRegex := regexp.MustCompile(`^enode://[0-9a-fA-F]{128}@.+:\d+(\?.*)?$`)
	if !enodeRegex.MatchString(bootnode) {
		return fmt.Errorf("invalid bootnode format (expected enode://pubkey@host:port): %s", bootnode)
	}

	return nil
}

// validateMetricsProtocol validates the metrics protocol
func validateMetricsProtocol(protocol string) error {
	validProtocols := map[string]bool{
		"HTTP":       true,
		"HTTPS":      true,
		"PROMETHEUS": true,
		"http":       true,
		"https":      true,
		"prometheus": true,
	}

	if !validProtocols[protocol] {
		return fmt.Errorf("invalid metrics protocol: %s (valid: HTTP, HTTPS, PROMETHEUS)", protocol)
	}

	return nil
}

// validateJWTAlgorithm validates the JWT authentication algorithm
func validateJWTAlgorithm(algorithm JWTAlgorithm) error {
	validAlgorithms := map[JWTAlgorithm]bool{
		JWTAlgorithmRS256: true,
		JWTAlgorithmRS384: true,
		JWTAlgorithmRS512: true,
		JWTAlgorithmES256: true,
		JWTAlgorithmES384: true,
		JWTAlgorithmES512: true,
	}

	if !validAlgorithms[algorithm] {
		return fmt.Errorf("invalid JWT algorithm: %s (valid: RS256, RS384, RS512, ES256, ES384, ES512)", algorithm)
	}

	return nil
}

// Validate validates the StartBesuOpts structure
func (opts *StartBesuOpts) Validate() error {
	var errors []string

	// Validate ID
	if err := validateID(opts.ID); err != nil {
		errors = append(errors, fmt.Sprintf("ID: %v", err))
	}

	// Validate P2P host
	if err := validateHost(opts.P2PHost); err != nil {
		errors = append(errors, fmt.Sprintf("P2P host: %v", err))
	}

	// Validate P2P port
	if err := validatePort(opts.P2PPort); err != nil {
		errors = append(errors, fmt.Sprintf("P2P port: %v", err))
	}

	// Validate RPC host
	if err := validateHost(opts.RPCHost); err != nil {
		errors = append(errors, fmt.Sprintf("RPC host: %v", err))
	}

	// Validate RPC port
	if err := validatePort(opts.RPCPort); err != nil {
		errors = append(errors, fmt.Sprintf("RPC port: %v", err))
	}

	// Validate consensus type
	if opts.ConsensusType != "" {
		if err := validateConsensusType(opts.ConsensusType); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate network ID
	if err := validateNetworkID(opts.NetworkID); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate chain ID
	if err := validateChainID(opts.ChainID); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate genesis file
	if err := validateGenesisFile(opts.GenesisFile); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate node private key
	if err := validateNodePrivateKey(opts.NodePrivateKey); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate miner address (optional)
	if opts.MinerAddress != "" {
		if err := validateEthereumAddress(opts.MinerAddress); err != nil {
			errors = append(errors, fmt.Sprintf("miner address: %v", err))
		}
	}

	// Validate bootnodes
	for i, bootnode := range opts.BootNodes {
		if err := validateBootnode(bootnode); err != nil {
			errors = append(errors, fmt.Sprintf("bootNodes[%d]: %v", i, err))
		}
	}

	// Validate version
	if err := validateVersion(opts.Version); err != nil {
		errors = append(errors, fmt.Sprintf("version: %v", err))
	}

	// Validate metrics configuration
	if opts.MetricsEnabled {
		if opts.MetricsPort <= 0 || opts.MetricsPort > 65535 {
			errors = append(errors, fmt.Sprintf("metrics port must be between 1 and 65535, got %d", opts.MetricsPort))
		}

		if err := validateMetricsProtocol(opts.MetricsProtocol); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate min gas price
	if opts.MinGasPrice < 0 {
		errors = append(errors, fmt.Sprintf("min gas price cannot be negative, got %d", opts.MinGasPrice))
	}

	// Validate JWT configuration
	if opts.JWTEnabled {
		if opts.JWTPublicKeyContent == "" {
			errors = append(errors, "JWT public key content cannot be empty when JWT is enabled")
		}

		if err := validateJWTAlgorithm(opts.JWTAuthenticationAlgorithm); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate permissions configuration
	for i, account := range opts.AccountsAllowList {
		if err := validateEthereumAddress(account); err != nil {
			errors = append(errors, fmt.Sprintf("accountsAllowList[%d]: %v", i, err))
		}
	}

	for i, node := range opts.NodesAllowList {
		if err := validateBootnode(node); err != nil {
			errors = append(errors, fmt.Sprintf("nodesAllowList[%d]: %v", i, err))
		}
	}

	// Check for port conflicts
	if err := opts.checkPortConflicts(); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("besu validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// checkPortConflicts checks if any ports are being reused
func (opts *StartBesuOpts) checkPortConflicts() error {
	ports := make(map[string]string)

	// Check P2P port
	if existingName, exists := ports[opts.P2PPort]; exists {
		return fmt.Errorf("port conflict: %s and P2P both use port %s", existingName, opts.P2PPort)
	}
	ports[opts.P2PPort] = "P2P"

	// Check RPC port
	if existingName, exists := ports[opts.RPCPort]; exists {
		return fmt.Errorf("port conflict: %s and RPC both use port %s", existingName, opts.RPCPort)
	}
	ports[opts.RPCPort] = "RPC"

	// Check metrics port if enabled
	if opts.MetricsEnabled {
		metricsPort := fmt.Sprintf("%d", opts.MetricsPort)
		if existingName, exists := ports[metricsPort]; exists {
			return fmt.Errorf("port conflict: %s and metrics both use port %s", existingName, metricsPort)
		}
		ports[metricsPort] = "metrics"
	}

	return nil
}
