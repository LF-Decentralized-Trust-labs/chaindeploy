package monitoring

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/certutils"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	nodes "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/chainlaunch/chainlaunch/pkg/notifications"
)

// Service defines the interface for the monitoring service
type Service interface {
	// Start begins monitoring nodes
	Start(ctx context.Context) error
	// Stop stops monitoring nodes
	Stop() error
	// AddNode adds a node to be monitored
	AddNode(node *Node) error
	// RemoveNode removes a node from monitoring
	RemoveNode(nodeID int64) error
	// NodeExists checks if a node exists
	NodeExists(nodeID int64) bool

	// GetNodeStatus returns the current status of a node
	GetNodeStatus(nodeID int64) (*NodeCheck, error)
	// GetAllNodeStatuses returns the current status of all nodes
	GetAllNodeStatuses() []*NodeCheck
}

// service implements the Service interface
type service struct {
	logger           *logger.Logger
	config           *Config
	nodes            map[int64]*Node
	nodesMutex       sync.RWMutex
	httpClient       *http.Client
	notificationSvc  notifications.Service
	stopChan         chan struct{}
	workerWaitGroup  sync.WaitGroup
	lastCheckResults map[int64]*NodeCheck
	resultsMutex     sync.RWMutex
	nodeService      *nodes.NodeService
}

// NewService creates a new monitoring service
func NewService(logger *logger.Logger, config *Config, notificationSvc notifications.Service, nodeService *nodes.NodeService) Service {
	if config == nil {
		config = DefaultConfig()
	}

	return &service{
		logger:           logger,
		config:           config,
		nodes:            make(map[int64]*Node),
		notificationSvc:  notificationSvc,
		stopChan:         make(chan struct{}),
		lastCheckResults: make(map[int64]*NodeCheck),
		httpClient: &http.Client{
			Timeout: config.DefaultTimeout,
		},
		nodeService: nodeService,
	}
}

// Start begins monitoring nodes
func (s *service) Start(ctx context.Context) error {
	// Create a worker pool to check nodes
	for i := 0; i < s.config.Workers; i++ {
		s.workerWaitGroup.Add(1)
		go s.worker(ctx, i)
	}
	return nil
}

// Stop stops monitoring nodes
func (s *service) Stop() error {
	close(s.stopChan)
	s.workerWaitGroup.Wait()
	return nil
}

// AddNode adds a node to be monitored
func (s *service) AddNode(node *Node) error {
	if node.ID == 0 {
		return fmt.Errorf("node ID cannot be zero")
	}
	if node.Endpoint == "" {
		return fmt.Errorf("node endpoint cannot be empty")
	}
	if node.Platform == "" {
		return fmt.Errorf("node platform cannot be empty")
	}

	// Set defaults if not provided
	if node.CheckInterval == 0 {
		node.CheckInterval = s.config.DefaultCheckInterval
	}
	if node.Timeout == 0 {
		node.Timeout = s.config.DefaultTimeout
	}
	if node.FailureThreshold == 0 {
		node.FailureThreshold = s.config.DefaultFailureThreshold
	}

	s.nodesMutex.Lock()
	s.nodes[node.ID] = node
	s.nodesMutex.Unlock()

	return nil
}

// RemoveNode removes a node from monitoring
func (s *service) RemoveNode(nodeID int64) error {
	s.nodesMutex.Lock()
	delete(s.nodes, nodeID)
	s.nodesMutex.Unlock()

	s.resultsMutex.Lock()
	delete(s.lastCheckResults, nodeID)
	s.resultsMutex.Unlock()

	return nil
}

func (s *service) NodeExists(nodeID int64) bool {
	s.nodesMutex.RLock()
	defer s.nodesMutex.RUnlock()
	_, exists := s.nodes[nodeID]
	return exists
}

// GetNodeStatus returns the current status of a node
func (s *service) GetNodeStatus(nodeID int64) (*NodeCheck, error) {
	s.resultsMutex.RLock()
	defer s.resultsMutex.RUnlock()

	result, exists := s.lastCheckResults[nodeID]
	if !exists {
		return nil, fmt.Errorf("node with ID %d not found", nodeID)
	}

	return result, nil
}

// GetAllNodeStatuses returns the current status of all nodes
func (s *service) GetAllNodeStatuses() []*NodeCheck {
	s.resultsMutex.RLock()
	defer s.resultsMutex.RUnlock()

	results := make([]*NodeCheck, 0, len(s.lastCheckResults))
	for _, result := range s.lastCheckResults {
		results = append(results, result)
	}

	return results
}

// worker is a background worker that checks nodes periodically
func (s *service) worker(ctx context.Context, workerID int) {
	defer s.workerWaitGroup.Done()

	ticker := time.NewTicker(s.config.DefaultCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkNodes(ctx)
		}
	}
}

// checkNodes performs the actual node checks
func (s *service) checkNodes(ctx context.Context) {

	now := time.Now()

	// Get a copy of the nodes to check
	s.nodesMutex.RLock()
	nodesToCheck := make([]*Node, 0)
	for _, node := range s.nodes {
		if now.Sub(node.LastChecked) >= node.CheckInterval {
			nodesToCheck = append(nodesToCheck, node)
		}
	}
	s.nodesMutex.RUnlock()

	// Check each node
	for _, node := range nodesToCheck {
		s.checkNode(ctx, node)
	}
}

// checkNode checks a single node and updates its status
func (s *service) checkNode(ctx context.Context, node *Node) {
	nodeResponse, err := s.nodeService.GetNode(ctx, node.ID)
	if err != nil {
		s.handleNodeCheckResult(node, NodeStatusDown, 0, err)
		return
	}

	var status NodeStatus
	var responseTime time.Duration
	var checkErr error

	// Route to appropriate check function based on node type
	switch {
	case nodeResponse.FabricPeer != nil:
		status, responseTime, checkErr = s.checkFabricPeer(ctx, node, nodeResponse.FabricPeer)
	case nodeResponse.FabricOrderer != nil:
		status, responseTime, checkErr = s.checkFabricOrderer(ctx, node, nodeResponse.FabricOrderer)
	case nodeResponse.BesuNode != nil:
		status, responseTime, checkErr = s.checkBesuNode(ctx, node, nodeResponse.BesuNode)
	default:
		checkErr = fmt.Errorf("unsupported node type")
	}

	if checkErr != nil {
		s.handleNodeCheckResult(node, NodeStatusDown, responseTime, checkErr)
		return
	}

	s.handleNodeCheckResult(node, status, responseTime, nil)
}

// checkFabricPeer checks a Fabric peer node using TLS only
func (s *service) checkFabricPeer(ctx context.Context, node *Node, peer *nodes.FabricPeerProperties) (NodeStatus, time.Duration, error) {
	start := time.Now()

	// Create a certificate pool for trusted CAs
	caCertPool := x509.NewCertPool()

	// Add TLS CA certificate if available
	if peer.TLSCACert != "" {
		caCert, err := certutils.ParseX509Certificate([]byte(peer.TLSCACert))
		if err != nil {
			return NodeStatusDown, 0, fmt.Errorf("failed to parse TLS CA certificate: %w", err)
		}
		caCertPool.AddCert(caCert)
	}

	// Get the node's TLS certificate
	var tlsCert tls.Certificate
	if peer.TLSCert != "" {
		x509Cert, err := certutils.ParseX509Certificate([]byte(peer.TLSCert))
		if err != nil {
			return NodeStatusDown, 0, fmt.Errorf("failed to parse TLS certificate: %w", err)
		}
		tlsCert = tls.Certificate{
			Certificate: [][]byte{x509Cert.Raw},
			PrivateKey:  nil,
		}
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	// Try TLS connection
	dialer := &net.Dialer{
		Timeout: node.Timeout,
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", node.Endpoint, tlsConfig)
	if err != nil {
		return NodeStatusDown, time.Since(start), err
	}
	defer conn.Close()

	// Verify the connection is established
	if err := conn.Handshake(); err != nil {
		return NodeStatusDown, time.Since(start), fmt.Errorf("TLS handshake failed: %w", err)
	}

	return NodeStatusUp, time.Since(start), nil
}

// checkFabricOrderer checks a Fabric orderer node using TLS only
func (s *service) checkFabricOrderer(ctx context.Context, node *Node, orderer *nodes.FabricOrdererProperties) (NodeStatus, time.Duration, error) {
	start := time.Now()

	// Create a certificate pool for trusted CAs
	caCertPool := x509.NewCertPool()

	// Add TLS CA certificate if available
	if orderer.TLSCACert != "" {
		caCert, err := certutils.ParseX509Certificate([]byte(orderer.TLSCACert))
		if err != nil {
			return NodeStatusDown, 0, fmt.Errorf("failed to parse TLS CA certificate: %w", err)
		}
		caCertPool.AddCert(caCert)
	}

	// Get the node's TLS certificate
	var tlsCert tls.Certificate
	if orderer.TLSCert != "" {
		x509Cert, err := certutils.ParseX509Certificate([]byte(orderer.TLSCert))
		if err != nil {
			return NodeStatusDown, 0, fmt.Errorf("failed to parse TLS certificate: %w", err)
		}
		tlsCert = tls.Certificate{
			Certificate: [][]byte{x509Cert.Raw},
			PrivateKey:  nil,
		}
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	// Try TLS connection
	dialer := &net.Dialer{
		Timeout: node.Timeout,
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", node.Endpoint, tlsConfig)
	if err != nil {
		return NodeStatusDown, time.Since(start), err
	}
	defer conn.Close()

	// Verify the connection is established
	if err := conn.Handshake(); err != nil {
		return NodeStatusDown, time.Since(start), fmt.Errorf("TLS handshake failed: %w", err)
	}

	return NodeStatusUp, time.Since(start), nil
}

// checkBesuNode checks a Besu node using JSON-RPC
func (s *service) checkBesuNode(ctx context.Context, node *Node, besu *nodes.BesuNodeProperties) (NodeStatus, time.Duration, error) {
	start := time.Now()

	// Create HTTP client
	client := &http.Client{
		Timeout: node.Timeout,
	}

	// Prepare JSON-RPC request
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "net_version",
		"params":  []interface{}{},
		"id":      1,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return NodeStatusDown, time.Since(start), fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
	}
	rpcUrl := fmt.Sprintf("http://%s:%d", besu.RPCHost, besu.RPCPort)
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", rpcUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return NodeStatusDown, time.Since(start), fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return NodeStatusDown, time.Since(start), fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NodeStatusDown, time.Since(start), fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON-RPC response
	var response struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  string `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return NodeStatusDown, time.Since(start), fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for RPC error
	if response.Error != nil {
		return NodeStatusDown, time.Since(start), fmt.Errorf("RPC error: %s (code: %d)", response.Error.Message, response.Error.Code)
	}

	// Verify we got a valid network version
	if response.Result == "" {
		return NodeStatusDown, time.Since(start), fmt.Errorf("invalid network version response")
	}

	return NodeStatusUp, time.Since(start), nil
}

// handleNodeCheckResult processes the result of a node check
func (s *service) handleNodeCheckResult(node *Node, status NodeStatus, responseTime time.Duration, err error) {
	now := time.Now()

	// Create the check result
	checkResult := &NodeCheck{
		Node:         node,
		Status:       status,
		ResponseTime: responseTime,
		Error:        err,
		Timestamp:    now,
	}

	// Update the node's status
	s.nodesMutex.Lock()

	// Capture the previous status for status change detection
	previousStatus := node.Status
	wasDown := previousStatus == NodeStatusDown

	// Update last checked time
	node.LastChecked = now

	// Handle status changes
	statusChanged := previousStatus != status
	if statusChanged {
		node.LastStatusChange = now
	}

	// Update failure count
	if status == NodeStatusDown {
		node.FailureCount++
	} else {
		node.FailureCount = 0
	}

	// Update node status
	node.Status = status

	// If node was down but is now up, record the recovery time
	var recoveryTime time.Time
	var downtimeDuration time.Duration
	if wasDown && status == NodeStatusUp {
		recoveryTime = now
		downtimeDuration = now.Sub(node.LastStatusChange)
	}

	s.nodesMutex.Unlock()

	// Store the check result
	s.resultsMutex.Lock()
	s.lastCheckResults[node.ID] = checkResult
	s.resultsMutex.Unlock()

	// Send notifications if needed
	ctx := context.Background()
	if status == NodeStatusDown && node.FailureCount >= node.FailureThreshold {
		s.sendNodeDownNotification(ctx, node, err)
	} else if statusChanged && status == NodeStatusUp && wasDown {
		// Node has recovered - send recovery notification
		s.sendNodeRecoveryNotification(ctx, node, responseTime, recoveryTime, downtimeDuration)
	}
}

// sendNodeDownNotification sends a notification that a node is down
func (s *service) sendNodeDownNotification(ctx context.Context, node *Node, err error) {
	data := notifications.NodeDowntimeData{
		NodeID:        node.ID,
		NodeName:      node.Name,
		NodeURL:       node.Endpoint,
		DownSince:     node.LastStatusChange,
		FailureCount:  node.FailureCount,
		Error:         err.Error(),
		ErrorMessage:  err.Error(),
		Endpoint:      node.Endpoint,
		NodeType:      node.Platform,
		LastSeen:      node.LastChecked,
		DowntimeStart: node.LastStatusChange,
	}

	// Send the notification
	if err := s.notificationSvc.SendNodeDowntimeNotification(ctx, data); err != nil {
		// Just log the error; we don't want to create a notification loop
		s.logger.Errorf("Failed to send node downtime notification: %v", err)
	}
}

// sendNodeRecoveryNotification sends a notification that a node has recovered
func (s *service) sendNodeRecoveryNotification(ctx context.Context, node *Node, responseTime time.Duration, recoveryTime time.Time, downtimeDuration time.Duration) {
	data := notifications.NodeUpData{
		NodeID:           node.ID,
		NodeName:         node.Name,
		NodeURL:          node.Endpoint,
		DownSince:        node.LastStatusChange,
		RecoveredAt:      recoveryTime,
		DowntimeDuration: downtimeDuration,
		ResponseTime:     responseTime,
	}

	// Send the notification
	if err := s.notificationSvc.SendNodeRecoveryNotification(ctx, data); err != nil {
		// Just log the error; we don't want to create a notification loop
		s.logger.Errorf("Failed to send node recovery notification: %v", err)
	}
}
