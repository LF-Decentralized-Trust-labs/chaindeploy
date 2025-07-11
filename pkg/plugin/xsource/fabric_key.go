package xsource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	key "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	ptypes "github.com/chainlaunch/chainlaunch/pkg/plugin/types"
)

type FabricKeyValue struct {
	BaseXSourceValue
	KeyID         int64
	OrgID         int64
	queries       *db.Queries
	nodeService   *nodeservice.NodeService
	keyManagement *key.KeyManagementService
}

// NewFabricKeyHandler creates a new handler for fabric-key x-source type
func NewFabricKeyHandler(queries *db.Queries, nodeService *nodeservice.NodeService, keyManagement *key.KeyManagementService) XSourceHandler {
	return &fabricKeyHandler{
		queries:       queries,
		nodeService:   nodeService,
		keyManagement: keyManagement,
	}
}

type fabricKeyHandler struct {
	queries       *db.Queries
	nodeService   *nodeservice.NodeService
	keyManagement *key.KeyManagementService
}

func (h *fabricKeyHandler) GetType() XSourceType {
	return FabricKey
}

func (h *fabricKeyHandler) CreateValue(key string, rawValue interface{}) (XSourceValue, error) {
	keyMap, ok := rawValue.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fabric-key value must be an object with keyId and orgId")
	}
	keyID, keyOk := keyMap["keyId"].(float64)
	orgID, orgOk := keyMap["orgId"].(float64)
	if !keyOk || !orgOk {
		return nil, fmt.Errorf("invalid fabric key format: both keyId and orgId are required")
	}

	return &FabricKeyValue{
		BaseXSourceValue: BaseXSourceValue{RawValue: rawValue, Key: key},
		KeyID:            int64(keyID),
		OrgID:            int64(orgID),
		queries:          h.queries,
		nodeService:      h.nodeService,
		keyManagement:    h.keyManagement,
	}, nil
}

func (h *fabricKeyHandler) ListOptions(ctx context.Context) ([]OptionItem, error) {
	value := &FabricKeyValue{
		queries:     h.queries,
		nodeService: h.nodeService,
	}
	return value.ListOptions(ctx)
}

func (v *FabricKeyValue) ListOptions(ctx context.Context) ([]OptionItem, error) {
	// TODO: Implement listing available keys
	return []OptionItem{}, nil
}

func (v *FabricKeyValue) Validate(ctx context.Context) error {
	// TODO: Implement validation
	return nil
}

// FabricKeyDetails represents the details of a Fabric key
type FabricKeyDetails struct {
	KeyID       int64
	OrgID       int64
	Name        string
	Type        string
	Description string
	MspID       string
	Certificate string
	PrivateKey  string
	CertPath    string // Path inside the container
	KeyPath     string // Path inside the container
}

func (v *FabricKeyValue) GetValue(ctx context.Context, spec ptypes.ParameterSpec) (interface{}, error) {
	// Get key details from key management service
	key, err := v.keyManagement.GetKey(ctx, int(v.KeyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Get organization details to get MSP ID
	org, err := v.queries.GetFabricOrganization(ctx, v.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Get private key
	privateKey, err := v.keyManagement.GetDecryptedPrivateKey(int(v.KeyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	// Create key details
	details := &FabricKeyDetails{
		KeyID:       v.KeyID,
		OrgID:       v.OrgID,
		Name:        key.Name,
		Type:        string(key.Algorithm),
		Description: *key.Description,
		MspID:       org.MspID,
		Certificate: *key.Certificate,
		PrivateKey:  privateKey,
		CertPath:    fmt.Sprintf("/etc/chainlaunch/%s/cert.pem", v.Key),
		KeyPath:     fmt.Sprintf("/etc/chainlaunch/%s/key.pem", v.Key),
	}

	return details, nil
}

func (v *FabricKeyValue) GetVolumeMounts(ctx context.Context) ([]VolumeMount, error) {
	var mounts []VolumeMount

	// Get key details
	key, err := v.keyManagement.GetKey(ctx, int(v.KeyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Get private key
	privateKey, err := v.keyManagement.GetDecryptedPrivateKey(int(v.KeyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	// Create a temporary directory for the key
	tempDir := fmt.Sprintf("/tmp/chainlaunch/key/%d", v.KeyID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create certificate file
	certPath := filepath.Join(tempDir, "cert.pem")
	if err := os.WriteFile(certPath, []byte(*key.Certificate), 0644); err != nil {
		return nil, fmt.Errorf("failed to write certificate: %w", err)
	}

	// Create private key file
	keyPath := filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(keyPath, []byte(privateKey), 0600); err != nil {
		return nil, fmt.Errorf("failed to write private key: %w", err)
	}

	// Add volume mounts
	mounts = append(mounts, VolumeMount{
		Source:      certPath,
		Target:      fmt.Sprintf("/etc/chainlaunch/%s/cert.pem", v.Key),
		Type:        "bind",
		ReadOnly:    true,
		Description: fmt.Sprintf("Fabric key certificate for %s", key.Name),
	})

	mounts = append(mounts, VolumeMount{
		Source:      keyPath,
		Target:      fmt.Sprintf("/etc/chainlaunch/%s/key.pem", v.Key),
		Type:        "bind",
		ReadOnly:    true,
		Description: fmt.Sprintf("Fabric private key for %s", key.Name),
	})

	return mounts, nil
}
