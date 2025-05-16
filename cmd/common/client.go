package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	defaultAPIURL = "http://localhost:8100/api/v1"
)

// Client represents a common API client
type Client struct {
	baseURL  string
	username string
	password string
}

// NewClientFromEnv creates a new client using environment variables
func NewClientFromEnv() (*Client, error) {
	apiURL := os.Getenv("CHAINLAUNCH_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	username := os.Getenv("CHAINLAUNCH_USER")
	if username == "" {
		return nil, fmt.Errorf("CHAINLAUNCH_USER environment variable is not set")
	}

	password := os.Getenv("CHAINLAUNCH_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("CHAINLAUNCH_PASSWORD environment variable is not set")
	}

	return &Client{
		baseURL:  apiURL,
		username: username,
		password: password,
	}, nil
}

// DoRequest performs an HTTP request with authentication
func (c *Client) DoRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	// Create HTTP request
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", c.baseURL, path), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// Get performs a GET request
func (c *Client) Get(path string) (*http.Response, error) {
	return c.DoRequest(http.MethodGet, path, nil)
}

// Post performs a POST request
func (c *Client) Post(path string, body interface{}) (*http.Response, error) {
	return c.DoRequest(http.MethodPost, path, body)
}

// Put performs a PUT request
func (c *Client) Put(path string, body interface{}) (*http.Response, error) {
	return c.DoRequest(http.MethodPut, path, body)
}

// Delete performs a DELETE request
func (c *Client) Delete(path string) (*http.Response, error) {
	return c.DoRequest(http.MethodDelete, path, nil)
}

// ReadBody reads and closes the response body
func ReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// CheckResponse checks if the response status code is in the expected range
func CheckResponse(resp *http.Response, expectedStatus ...int) error {
	for _, status := range expectedStatus {
		if resp.StatusCode == status {
			return nil
		}
	}

	body, _ := ReadBody(resp)
	return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
}
