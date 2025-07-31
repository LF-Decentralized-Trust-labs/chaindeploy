package common

import (
	"encoding/json"
	"fmt"
	"net/http"

	httptypes "github.com/chainlaunch/chainlaunch/pkg/networks/http"
)

// CreateFabricNetwork creates a new Fabric network using the REST API
func (c *Client) CreateFabricNetwork(req *httptypes.CreateFabricNetworkRequest) (*httptypes.NetworkResponse, error) {
	resp, err := c.Post("/networks/fabric", req)
	if err != nil {
		return nil, fmt.Errorf("failed to create fabric network: %w", err)
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp, http.StatusCreated); err != nil {
		return nil, err
	}
	var network httptypes.NetworkResponse
	if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &network, nil
}

// CreateBesuNetwork creates a new Besu network using the API and returns the BesuNetworkResponse.
func (c *Client) CreateBesuNetwork(req *httptypes.CreateBesuNetworkRequest) (*httptypes.BesuNetworkResponse, error) {
	resp, err := c.Post("/networks/besu", req)
	if err != nil {
		return nil, fmt.Errorf("failed to create besu network: %w", err)
	}
	if err := CheckResponse(resp, 200, 201); err != nil {
		return nil, err
	}
	var netResp httptypes.BesuNetworkResponse
	body, err := ReadBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if err := json.Unmarshal(body, &netResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &netResp, nil
}

// JoinPeerToFabricNetwork joins a peer to a Fabric network using the REST API
func (c *Client) JoinPeerToFabricNetwork(networkID, peerID int64) (*httptypes.NetworkResponse, error) {
	path := fmt.Sprintf("/networks/fabric/%d/peers/%d/join", networkID, peerID)
	resp, err := c.Post(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to join peer %d to network %d: %w", peerID, networkID, err)
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp, http.StatusOK); err != nil {
		return nil, err
	}
	var network httptypes.NetworkResponse
	if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &network, nil
}

// JoinAllPeersToFabricNetwork joins all peer nodes to a Fabric network
func (c *Client) JoinAllPeersToFabricNetwork(networkID int64, peerNodeIDs []int64) ([]*httptypes.NetworkResponse, []error) {
	var results []*httptypes.NetworkResponse
	var errs []error
	for _, peerID := range peerNodeIDs {
		resp, err := c.JoinPeerToFabricNetwork(networkID, peerID)
		if err != nil {
			errs = append(errs, fmt.Errorf("peer %d: %w", peerID, err))
		} else {
			results = append(results, resp)
		}
	}
	return results, errs
}

// JoinOrdererToFabricNetwork joins an orderer to a Fabric network using the REST API
func (c *Client) JoinOrdererToFabricNetwork(networkID, ordererID int64) (*httptypes.NetworkResponse, error) {
	path := fmt.Sprintf("/networks/fabric/%d/orderers/%d/join", networkID, ordererID)
	resp, err := c.Post(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to join orderer %d to network %d: %w", ordererID, networkID, err)
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp, http.StatusOK); err != nil {
		return nil, err
	}
	var network httptypes.NetworkResponse
	if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &network, nil
}

// JoinAllOrderersToFabricNetwork joins all orderer nodes to a Fabric network
func (c *Client) JoinAllOrderersToFabricNetwork(networkID int64, ordererNodeIDs []int64) ([]*httptypes.NetworkResponse, []error) {
	var results []*httptypes.NetworkResponse
	var errs []error
	for _, ordererID := range ordererNodeIDs {
		resp, err := c.JoinOrdererToFabricNetwork(networkID, ordererID)
		if err != nil {
			errs = append(errs, fmt.Errorf("orderer %d: %w", ordererID, err))
		} else {
			results = append(results, resp)
		}
	}
	return results, errs
}

// ListFabricNetworks lists all Fabric networks
func (c *Client) ListFabricNetworks() (*httptypes.ListNetworksResponse, error) {
	path := "/networks/fabric"
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp, http.StatusOK); err != nil {
		return nil, err
	}
	var result httptypes.ListNetworksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListBesuNetworks() (*httptypes.ListNetworksResponse, error) {
	path := "/networks/besu"
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := CheckResponse(resp, http.StatusOK); err != nil {
		return nil, err
	}
	var result httptypes.ListNetworksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
