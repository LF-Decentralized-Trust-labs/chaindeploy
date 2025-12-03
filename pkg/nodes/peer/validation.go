package peer

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// validatePort checks if a port string is valid (1-65535)
func validatePort(port string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}
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

// validateAddress validates a listen address in format host:port
func validateAddress(address string, fieldName string) error {
	if address == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid %s format (expected host:port): %w", fieldName, err)
	}

	if host == "" {
		return fmt.Errorf("%s host cannot be empty", fieldName)
	}

	if err := validatePort(port); err != nil {
		return fmt.Errorf("%s %w", fieldName, err)
	}

	return nil
}

// validateDomainName validates a single domain name or IP address
func validateDomainName(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain name cannot be empty")
	}

	// Check if it's a valid IP address
	if ip := net.ParseIP(domain); ip != nil {
		return nil
	}

	// Check if it's a valid domain name
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain name: %s", domain)
	}

	return nil
}

// validateID validates a node ID
func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("node ID cannot be empty")
	}

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

// validateVersion validates a Fabric version string
func validateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Basic semantic version check (v2.5.0, 2.5.0, etc.)
	versionRegex := regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?(-\w+)?$`)
	if !versionRegex.MatchString(version) {
		return fmt.Errorf("invalid version format: %s (expected format: v2.5.0 or 2.5.0)", version)
	}

	return nil
}

// Validate validates the StartPeerOpts structure
func (opts *StartPeerOpts) Validate() error {
	var errors []string

	// Validate ID
	if err := validateID(opts.ID); err != nil {
		errors = append(errors, fmt.Sprintf("ID: %v", err))
	}

	// Validate listen address
	if err := validateAddress(opts.ListenAddress, "listenAddress"); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate chaincode address
	if err := validateAddress(opts.ChaincodeAddress, "chaincodeAddress"); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate events address
	if err := validateAddress(opts.EventsAddress, "eventsAddress"); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate operations listen address
	if err := validateAddress(opts.OperationsListenAddress, "operationsListenAddress"); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate external endpoint (optional but if provided must be valid)
	if opts.ExternalEndpoint != "" {
		if err := validateAddress(opts.ExternalEndpoint, "externalEndpoint"); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate domain names
	for i, domain := range opts.DomainNames {
		if err := validateDomainName(domain); err != nil {
			errors = append(errors, fmt.Sprintf("domainNames[%d]: %v", i, err))
		}
	}

	// Validate version
	if err := validateVersion(opts.Version); err != nil {
		errors = append(errors, fmt.Sprintf("version: %v", err))
	}

	// Validate address overrides
	for i, override := range opts.AddressOverrides {
		if override.From == "" {
			errors = append(errors, fmt.Sprintf("addressOverrides[%d]: 'from' address cannot be empty", i))
		} else if err := validateAddress(override.From, fmt.Sprintf("addressOverrides[%d].from", i)); err != nil {
			errors = append(errors, err.Error())
		}

		if override.To == "" {
			errors = append(errors, fmt.Sprintf("addressOverrides[%d]: 'to' address cannot be empty", i))
		} else if err := validateAddress(override.To, fmt.Sprintf("addressOverrides[%d].to", i)); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Check for port conflicts
	if err := opts.checkPortConflicts(); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("peer validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// checkPortConflicts checks if any ports are being reused
func (opts *StartPeerOpts) checkPortConflicts() error {
	ports := make(map[string]string)

	addresses := map[string]string{
		"listenAddress":           opts.ListenAddress,
		"chaincodeAddress":        opts.ChaincodeAddress,
		"eventsAddress":           opts.EventsAddress,
		"operationsListenAddress": opts.OperationsListenAddress,
	}

	for name, address := range addresses {
		if address == "" {
			continue
		}

		_, port, err := net.SplitHostPort(address)
		if err != nil {
			continue // Will be caught by address validation
		}

		if existingName, exists := ports[port]; exists {
			return fmt.Errorf("port conflict: %s and %s both use port %s", existingName, name, port)
		}
		ports[port] = name
	}

	return nil
}
