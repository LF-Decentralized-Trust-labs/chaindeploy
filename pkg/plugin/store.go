package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/chainlaunch/chainlaunch/pkg/plugin/registry"
	"github.com/chainlaunch/chainlaunch/pkg/plugin/types"
)

// Store defines the interface for plugin storage operations
type Store interface {
	CreatePlugin(ctx context.Context, plugin *types.Plugin) error
	GetPlugin(ctx context.Context, name string) (*types.Plugin, error)
	ListPlugins(ctx context.Context) ([]*types.Plugin, error)
	DeletePlugin(ctx context.Context, name string) error
	UpdatePlugin(ctx context.Context, plugin *types.Plugin) error
	UpdateDeploymentMetadata(ctx context.Context, name string, metadata map[string]interface{}) error
	UpdateDeploymentStatus(ctx context.Context, name string, status string) error
	GetDeploymentMetadata(ctx context.Context, name string) (map[string]interface{}, error)
	GetDeploymentStatus(ctx context.Context, name string) (string, error)
	ListKeyStoreIDs(ctx context.Context) ([]string, error)
	ListFabricOrgs(ctx context.Context) ([]string, error)
	ListKeyStoreOptions(ctx context.Context) ([]types.OptionItem, error)
	ListFabricOrgOptions(ctx context.Context) ([]types.OptionItem, error)
	ListFabricKeyOptions(ctx context.Context) ([]types.OptionItem, error)
	// New methods for fetching details
	GetFabricPeerDetails(ctx context.Context, id string) (*types.FabricPeerDetails, error)
	GetFabricOrgDetails(ctx context.Context, id string) (*types.FabricOrgDetails, error)
	GetKeyDetails(ctx context.Context, id string) (*types.KeyDetails, error)
	GetFabricKeyDetails(ctx context.Context, keyIdString string, orgIdString string) (*types.FabricKeyDetails, error)
}

// Service represents a docker-compose service
type Service struct {
	Name        string                 `json:"name"`
	Image       string                 `json:"image"`
	Ports       []string               `json:"ports,omitempty"`
	Environment map[string]string      `json:"environment,omitempty"`
	Volumes     []string               `json:"volumes,omitempty"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// ServiceStatus extends Service with runtime information
type ServiceStatus struct {
	Service
	State      string   `json:"state"`
	Running    bool     `json:"running"`
	Health     string   `json:"health,omitempty"`
	Containers []string `json:"containers,omitempty"`
	LastError  string   `json:"last_error,omitempty"`
	CreatedAt  string   `json:"created_at,omitempty"`
	StartedAt  string   `json:"started_at,omitempty"`
}

// SQLStore implements the Store interface using SQL database
type SQLStore struct {
	queries     *db.Queries
	nodeService *nodeservice.NodeService
}

// NewSQLStore creates a new SQL store
func NewSQLStore(db *db.Queries, nodeService *nodeservice.NodeService) *SQLStore {
	return &SQLStore{
		queries:     db,
		nodeService: nodeService,
	}
}

// getDeploymentStatusFromDB extracts deployment status from database plugin data
func (s *SQLStore) getDeploymentStatusFromDB(dbPlugin *db.Plugin) *types.DeploymentStatus {
	// If no deployment metadata, return not deployed status
	if dbPlugin.DeploymentMetadata == nil {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	metadataBytes, ok := dbPlugin.DeploymentMetadata.([]byte)
	if !ok {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	var deploymentMetadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &deploymentMetadata); err != nil {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	// Extract values from deployment metadata
	deployedAt, ok := deploymentMetadata["deployedAt"].(string)
	if !ok {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	startedAt, err := time.Parse(time.RFC3339, deployedAt)
	if err != nil {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	parametersRaw, ok := deploymentMetadata["parameters"]
	if !ok {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	parameters, ok := parametersRaw.(map[string]interface{})
	if !ok {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	projectNameRaw, ok := deploymentMetadata["projectName"]
	if !ok {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	projectName, ok := projectNameRaw.(string)
	if !ok {
		return &types.DeploymentStatus{
			Status: "not deployed",
		}
	}

	// Create deployment status
	return &types.DeploymentStatus{
		Status:      dbPlugin.DeploymentStatus.String,
		StartedAt:   startedAt,
		ProjectName: projectName,
		Parameters:  parameters,
	}
}

// CreatePlugin creates a new plugin in the database
func (s *SQLStore) CreatePlugin(ctx context.Context, plugin *types.Plugin) error {
	metadataJSON, err := json.Marshal(plugin.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	specJSON, err := json.Marshal(plugin.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	_, err = s.queries.CreatePlugin(ctx, &db.CreatePluginParams{
		Name:       plugin.Metadata.Name,
		ApiVersion: plugin.APIVersion,
		Kind:       plugin.Kind,
		Metadata:   metadataJSON,
		Spec:       specJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to create plugin: %w", err)
	}
	return nil
}

// GetPlugin retrieves a plugin by name
func (s *SQLStore) GetPlugin(ctx context.Context, name string) (*types.Plugin, error) {
	dbPlugin, err := s.queries.GetPlugin(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin: %w", err)
	}

	metadataBytes, ok := dbPlugin.Metadata.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid metadata type: expected []byte")
	}

	var metadata types.Metadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	specBytes, ok := dbPlugin.Spec.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid spec type: expected []byte")
	}

	var spec types.Spec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal spec: %w", err)
	}

	plugin := &types.Plugin{
		APIVersion: dbPlugin.ApiVersion,
		Kind:       dbPlugin.Kind,
		Metadata:   metadata,
		Spec:       spec,
	}

	// Get deployment status
	plugin.DeploymentStatus = s.getDeploymentStatusFromDB(dbPlugin)

	return plugin, nil
}

// ListPlugins retrieves all plugins
func (s *SQLStore) ListPlugins(ctx context.Context) ([]*types.Plugin, error) {
	dbPlugins, err := s.queries.ListPlugins(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %w", err)
	}

	plugins := make([]*types.Plugin, len(dbPlugins))
	for i, dbPlugin := range dbPlugins {
		metadataBytes, ok := dbPlugin.Metadata.([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid metadata type: expected []byte")
		}

		var metadata types.Metadata
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		specBytes, ok := dbPlugin.Spec.([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid spec type: expected []byte")
		}

		var spec types.Spec
		if err := json.Unmarshal(specBytes, &spec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal spec: %w", err)
		}

		plugin := &types.Plugin{
			APIVersion: dbPlugin.ApiVersion,
			Kind:       dbPlugin.Kind,
			Metadata:   metadata,
			Spec:       spec,
		}

		// Get deployment status
		plugin.DeploymentStatus = s.getDeploymentStatusFromDB(dbPlugin)

		plugins[i] = plugin
	}

	return plugins, nil
}

// DeletePlugin removes a plugin by name
func (s *SQLStore) DeletePlugin(ctx context.Context, name string) error {
	err := s.queries.DeletePlugin(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to delete plugin: %w", err)
	}
	return nil
}

// UpdatePlugin updates an existing plugin
func (s *SQLStore) UpdatePlugin(ctx context.Context, plugin *types.Plugin) error {
	// Get existing plugin to preserve deployment status
	existingPlugin, err := s.GetPlugin(ctx, plugin.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to get existing plugin: %w", err)
	}

	metadataJSON, err := json.Marshal(plugin.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	specJSON, err := json.Marshal(plugin.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	_, err = s.queries.UpdatePlugin(ctx, &db.UpdatePluginParams{
		ApiVersion: plugin.APIVersion,
		Kind:       plugin.Kind,
		Metadata:   metadataJSON,
		Spec:       specJSON,
		Name:       plugin.Metadata.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to update plugin: %w", err)
	}

	// Preserve deployment status
	if existingPlugin.DeploymentStatus != nil {
		plugin.DeploymentStatus = existingPlugin.DeploymentStatus
	}

	return nil
}

// UpdateDeploymentMetadata updates the deployment metadata for a plugin
func (s *SQLStore) UpdateDeploymentMetadata(ctx context.Context, name string, metadata map[string]interface{}) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment metadata: %w", err)
	}

	err = s.queries.UpdateDeploymentMetadata(ctx, &db.UpdateDeploymentMetadataParams{
		Name:               name,
		DeploymentMetadata: metadataJSON,
	})
	if err != nil {
		return fmt.Errorf("failed to update deployment metadata: %w", err)
	}
	return nil
}

// UpdateDeploymentStatus updates the deployment status for a plugin
func (s *SQLStore) UpdateDeploymentStatus(ctx context.Context, name string, status string) error {
	err := s.queries.UpdateDeploymentStatus(ctx, &db.UpdateDeploymentStatusParams{
		Name:             name,
		DeploymentStatus: sql.NullString{String: status, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}
	return nil
}

// GetDeploymentMetadata retrieves the deployment metadata for a plugin
func (s *SQLStore) GetDeploymentMetadata(ctx context.Context, name string) (map[string]interface{}, error) {
	metadata, err := s.queries.GetDeploymentMetadata(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment metadata: %w", err)
	}

	metadataBytes, ok := metadata.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid deployment metadata type: expected []byte")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
	}

	return result, nil
}

// GetDeploymentStatus retrieves the deployment status for a plugin
func (s *SQLStore) GetDeploymentStatus(ctx context.Context, name string) (string, error) {
	status, err := s.queries.GetDeploymentStatus(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to get deployment status: %w", err)
	}
	return status.String, nil
}

// ListKeyStoreIDs fetches valid key IDs for x-source validation
func (s *SQLStore) ListKeyStoreIDs(ctx context.Context) ([]string, error) {
	// TODO: Query your DB for available key IDs
	return []string{"key1", "key2"}, nil
}

// ListFabricOrgs fetches valid Fabric orgs for x-source validation
func (s *SQLStore) ListFabricOrgs(ctx context.Context) ([]string, error) {
	// TODO: Query your DB for available Fabric orgs
	return []string{"orga", "orgb"}, nil
}

func (s *SQLStore) ListKeyStoreOptions(ctx context.Context) ([]types.OptionItem, error) {
	rows, err := s.queries.ListKeys(ctx, &db.ListKeysParams{Limit: 100, Offset: 0})
	if err != nil {
		return nil, err
	}
	opts := make([]types.OptionItem, len(rows))
	for i, row := range rows {
		opts[i] = types.OptionItem{
			Label: row.Name,                  // Show key name as label
			Value: fmt.Sprintf("%d", row.ID), // Use key ID as value
		}
	}
	return opts, nil
}

func (s *SQLStore) ListFabricOrgOptions(ctx context.Context) ([]types.OptionItem, error) {
	rows, err := s.queries.ListFabricOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	opts := make([]types.OptionItem, len(rows))
	for i, row := range rows {
		opts[i] = types.OptionItem{
			Label: row.MspID,                 // Show MSP ID as label (or row.Description.String if you want description)
			Value: fmt.Sprintf("%d", row.ID), // Use org ID as value
		}
	}
	return opts, nil
}

// GetFabricPeerDetails retrieves details for a Fabric peer
func (s *SQLStore) GetFabricPeerDetails(ctx context.Context, id string) (*types.FabricPeerDetails, error) {
	peerID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid peer ID format: %w", err)
	}

	peer, err := s.nodeService.GetNode(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}
	if peer.FabricPeer == nil {
		return nil, fmt.Errorf("peer is not a Fabric peer")
	}

	return &types.FabricPeerDetails{
		ID:               id,
		Name:             peer.Name,
		ExternalEndpoint: peer.FabricPeer.ExternalEndpoint,
		TLSCert:          peer.FabricPeer.TLSCert,
		TLSCACert:        peer.FabricPeer.TLSCACert,
		MspID:            peer.FabricPeer.MSPID,
		OrgID:            peer.FabricPeer.OrganizationID,
	}, nil
}

// GetFabricOrgDetails retrieves details for a Fabric org
func (s *SQLStore) GetFabricOrgDetails(ctx context.Context, id string) (*types.FabricOrgDetails, error) {
	orgID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid org ID format: %w", err)
	}

	org, err := s.queries.GetFabricOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get org: %w", err)
	}

	return &types.FabricOrgDetails{
		ID:          orgID,
		MspID:       org.MspID,
		Description: org.Description.String,
	}, nil
}

// GetKeyDetails retrieves details for a key
func (s *SQLStore) GetKeyDetails(ctx context.Context, id string) (*types.KeyDetails, error) {
	keyID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid key ID format: %w", err)
	}

	key, err := s.queries.GetKey(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	return &types.KeyDetails{
		ID:          keyID,
		Name:        key.Name,
		Type:        key.Algorithm,
		Description: key.Description.String,
	}, nil
}

// GetFabricKeyDetails retrieves details for a Fabric key
func (s *SQLStore) GetFabricKeyDetails(ctx context.Context, keyIdString string, orgIdString string) (*types.FabricKeyDetails, error) {
	keyID, err := strconv.ParseInt(keyIdString, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid key ID format: %w", err)
	}
	orgID, err := strconv.ParseInt(orgIdString, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid org ID format: %w", err)
	}
	key, err := s.queries.GetKey(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Get the organization details for this key
	org, err := s.queries.GetFabricOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization for key: %w", err)
	}

	return &types.FabricKeyDetails{
		KeyID:       keyID,
		Name:        key.Name,
		Type:        key.Algorithm,
		Description: key.Description.String,
		MspID:       org.MspID,
		Certificate: key.Certificate.String,
	}, nil
}

func (s *SQLStore) ListFabricKeyOptions(ctx context.Context) ([]types.OptionItem, error) {
	// Get all keys that have certificates (which are likely to be Fabric keys)
	rows, err := s.queries.ListKeys(ctx, &db.ListKeysParams{Limit: 100, Offset: 0})
	if err != nil {
		return nil, err
	}

	// Get all Fabric organizations
	orgs, err := s.queries.ListFabricOrganizations(ctx)
	if err != nil {
		return nil, err
	}

	// Create a map of org IDs to MSP IDs for quick lookup
	orgMap := make(map[int64]string)
	for _, org := range orgs {
		orgMap[org.ID] = org.MspID
	}

	// Create options for each key-org combination
	var opts []types.OptionItem
	for _, key := range rows {
		if key.Certificate.Valid {
			for orgID, mspID := range orgMap {
				opts = append(opts, types.OptionItem{
					Label: fmt.Sprintf("%s (%s)", key.Name, mspID),
					Value: fmt.Sprintf("%d:%d", key.ID, orgID),
				})
			}
		}
	}
	return opts, nil
}

// RegistryStore aggregates plugins from the SQL store and the plugin registry
// It implements Store and can be used in the handler to support all sources

type RegistryStore struct {
	SQLStore *SQLStore
	Registry *registry.Registry
}

func NewRegistryStore(sqlStore *SQLStore, reg *registry.Registry) *RegistryStore {
	return &RegistryStore{
		SQLStore: sqlStore,
		Registry: reg,
	}
}

func (s *RegistryStore) CreatePlugin(ctx context.Context, plugin *types.Plugin) error {
	return s.SQLStore.CreatePlugin(ctx, plugin)
}

func (s *RegistryStore) GetPlugin(ctx context.Context, name string) (*types.Plugin, error) {
	// Only fetch from SQLStore (installed plugins)
	return s.SQLStore.GetPlugin(ctx, name)
}

func (s *RegistryStore) ListPlugins(ctx context.Context) ([]*types.Plugin, error) {
	// Only fetch from SQLStore (installed plugins)
	return s.SQLStore.ListPlugins(ctx)
}

func (s *RegistryStore) DeletePlugin(ctx context.Context, name string) error {
	return s.SQLStore.DeletePlugin(ctx, name)
}

func (s *RegistryStore) UpdatePlugin(ctx context.Context, plugin *types.Plugin) error {
	return s.SQLStore.UpdatePlugin(ctx, plugin)
}

func (s *RegistryStore) GetDeploymentMetadata(ctx context.Context, name string) (map[string]interface{}, error) {
	return s.SQLStore.GetDeploymentMetadata(ctx, name)
}

func (s *RegistryStore) GetDeploymentStatus(ctx context.Context, name string) (string, error) {
	return s.SQLStore.GetDeploymentStatus(ctx, name)
}

func (s *RegistryStore) GetFabricKeyDetails(ctx context.Context, keyIdString string, orgIdString string) (*types.FabricKeyDetails, error) {
	return s.SQLStore.GetFabricKeyDetails(ctx, keyIdString, orgIdString)
}

func (s *RegistryStore) GetFabricPeerDetails(ctx context.Context, id string) (*types.FabricPeerDetails, error) {
	return s.SQLStore.GetFabricPeerDetails(ctx, id)
}

func (s *RegistryStore) GetFabricOrgDetails(ctx context.Context, id string) (*types.FabricOrgDetails, error) {
	return s.SQLStore.GetFabricOrgDetails(ctx, id)
}

func (s *RegistryStore) ListKeyStoreIDs(ctx context.Context) ([]string, error) {
	return s.SQLStore.ListKeyStoreIDs(ctx)
}

func (s *RegistryStore) ListFabricOrgs(ctx context.Context) ([]string, error) {
	return s.SQLStore.ListFabricOrgs(ctx)
}

func (s *RegistryStore) ListKeyStoreOptions(ctx context.Context) ([]types.OptionItem, error) {
	return s.SQLStore.ListKeyStoreOptions(ctx)
}

func (s *RegistryStore) ListFabricOrgOptions(ctx context.Context) ([]types.OptionItem, error) {
	return s.SQLStore.ListFabricOrgOptions(ctx)
}

func (s *RegistryStore) ListFabricKeyOptions(ctx context.Context) ([]types.OptionItem, error) {
	return s.SQLStore.ListFabricKeyOptions(ctx)
}

func (s *RegistryStore) GetKeyDetails(ctx context.Context, id string) (*types.KeyDetails, error) {
	return s.SQLStore.GetKeyDetails(ctx, id)
}

func (s *RegistryStore) UpdateDeploymentMetadata(ctx context.Context, name string, metadata map[string]interface{}) error {
	return s.SQLStore.UpdateDeploymentMetadata(ctx, name, metadata)
}

func (s *RegistryStore) UpdateDeploymentStatus(ctx context.Context, name string, status string) error {
	return s.SQLStore.UpdateDeploymentStatus(ctx, name, status)
}
