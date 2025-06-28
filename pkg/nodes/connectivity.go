package nodes

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// ConnectivityCheckResult holds the result of a connectivity check
// If Error is nil, the check succeeded
// Latency is the time to establish the connection (and TLS handshake if enabled)
type ConnectivityCheckResult struct {
	Success      bool
	Latency      time.Duration
	Error        error
	TLSHandshake bool // true if TLS handshake was attempted
	TLSSuccess   bool // true if TLS handshake succeeded
}

// CheckNodeConnectivity checks TCP connectivity to host:port, optionally with TLS handshake
// If useTLS is true, tlsConfig may be nil (in which case default config is used)
func CheckNodeConnectivity(host string, port int, useTLS bool, tlsConfig *tls.Config) ConnectivityCheckResult {
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	start := time.Now()
	var result ConnectivityCheckResult

	if useTLS {
		result.TLSHandshake = true
		if tlsConfig == nil {
			tlsConfig = &tls.Config{InsecureSkipVerify: true} // Accept any cert for connectivity test
		}
		conn, err := tls.Dial("tcp", address, tlsConfig)
		result.Latency = time.Since(start)
		if err != nil {
			result.Error = fmt.Errorf("TLS dial failed: %w", err)
			return result
		}
		result.TLSSuccess = true
		result.Success = true
		_ = conn.Close()
		return result
	}

	// Plain TCP
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	result.Latency = time.Since(start)
	if err != nil {
		result.Error = fmt.Errorf("TCP dial failed: %w", err)
		return result
	}
	result.Success = true
	_ = conn.Close()
	return result
}
