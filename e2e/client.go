package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"os"

	orgtypes "github.com/chainlaunch/chainlaunch/pkg/fabric/handler"
	networkshttp "github.com/chainlaunch/chainlaunch/pkg/networks/http"
	nodeshttp "github.com/chainlaunch/chainlaunch/pkg/nodes/http"
)

const (
	defaultAPIURL = "http://localhost:8080/api/v1"
)

// TestClient represents a test API client
type TestClient struct {
	baseURL  string
	username string
	password string
}

// NewTestClient creates a new test client using environment variables
func NewTestClient() (*TestClient, error) {
	apiURL := os.Getenv("API_BASE_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	username := os.Getenv("API_USERNAME")
	if username == "" {
		return nil, fmt.Errorf("API_USERNAME environment variable is not set")
	}

	password := os.Getenv("API_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("API_PASSWORD environment variable is not set")
	}

	return &TestClient{
		baseURL:  apiURL,
		username: username,
		password: password,
	}, nil
}

// DoRequest performs an HTTP request with authentication
func (c *TestClient) DoRequest(method, path string, body interface{}) (*nethttp.Response, error) {
	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	// Create HTTP request
	req, err := nethttp.NewRequest(method, fmt.Sprintf("%s%s", c.baseURL, path), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Send request
	client := &nethttp.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// CreateNode creates a new node
func (c *TestClient) CreateNode(req *nodeshttp.CreateNodeRequest) (*nodeshttp.NodeResponse, error) {
	resp, err := c.DoRequest(nethttp.MethodPost, "/nodes", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var nodeResp nodeshttp.NodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&nodeResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &nodeResp, nil
}

// CreateOrganization creates a new fabric organization
func (c *TestClient) CreateOrganization(req *orgtypes.CreateOrganizationRequest) (*orgtypes.OrganizationResponse, error) {
	resp, err := c.DoRequest(nethttp.MethodPost, "/organizations", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var orgResp orgtypes.OrganizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&orgResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &orgResp, nil
}

// AddNodeToNetwork adds a node to a network
func (c *TestClient) AddNodeToNetwork(networkID int64, req *networkshttp.AddNodeToNetworkRequest) error {
	resp, err := c.DoRequest(nethttp.MethodPost, fmt.Sprintf("/networks/%d/nodes", networkID), req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetNetwork retrieves a network by ID
func (c *TestClient) GetNetwork(id int64) (*networkshttp.NetworkResponse, error) {
	resp, err := c.DoRequest(nethttp.MethodGet, fmt.Sprintf("/networks/%d", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var networkResp networkshttp.NetworkResponse
	if err := json.NewDecoder(resp.Body).Decode(&networkResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &networkResp, nil
}

// DeleteNetwork deletes a network by ID
func (c *TestClient) DeleteNetwork(id int64) error {
	resp, err := c.DoRequest(nethttp.MethodDelete, fmt.Sprintf("/networks/%d", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListNetworks retrieves a list of networks
func (c *TestClient) ListNetworks() (*networkshttp.ListNetworksResponse, error) {
	resp, err := c.DoRequest(nethttp.MethodGet, "/networks", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var listResp networkshttp.ListNetworksResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}
