package chainlaunchdeploy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/common/ports"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	keymgmtservice "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/hyperledger/fabric-admin-sdk/pkg/chaincode"
	fabricclient "github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"google.golang.org/grpc/status"
)

// --- Service-layer structs ---
type Chaincode struct {
	ID              int64                 `json:"id"`
	Name            string                `json:"name"`
	NetworkID       int64                 `json:"network_id"`
	NetworkName     string                `json:"network_name"`     // Name of the network
	NetworkPlatform string                `json:"network_platform"` // Platform/type (fabric/besu/etc)
	CreatedAt       string                `json:"created_at"`       // ISO8601
	Definitions     []ChaincodeDefinition `json:"definitions"`
}

type ChaincodeDefinition struct {
	ID                int64  `json:"id"`
	ChaincodeID       int64  `json:"chaincode_id"`
	Version           string `json:"version"`
	Sequence          int64  `json:"sequence"`
	DockerImage       string `json:"docker_image"`
	EndorsementPolicy string `json:"endorsement_policy"`
	ChaincodeAddress  string `json:"chaincode_address"`
	CreatedAt         string `json:"created_at"` // ISO8601
}

type PeerStatus struct {
	ID           int64  `json:"id"`
	DefinitionID int64  `json:"definition_id"`
	PeerID       int64  `json:"peer_id"`
	Status       string `json:"status"`
	LastUpdated  string `json:"last_updated"` // ISO8601
}

// ChaincodeService handles chaincode operations
type ChaincodeService struct {
	queries              *db.Queries
	logger               *logger.Logger
	nodeService          *service.NodeService
	keyManagementService *keymgmtservice.KeyManagementService
}

// NewChaincodeService creates a new chaincode service
func NewChaincodeService(queries *db.Queries, logger *logger.Logger, nodeService *service.NodeService, keyManagementService *keymgmtservice.KeyManagementService) *ChaincodeService {
	return &ChaincodeService{
		queries:              queries,
		logger:               logger,
		nodeService:          nodeService,
		keyManagementService: keyManagementService,
	}
}

// --- Chaincode CRUD ---
func (s *ChaincodeService) CreateChaincode(ctx context.Context, name string, networkID int64) (*Chaincode, error) {
	cc, err := s.queries.CreateChaincode(ctx, &db.CreateChaincodeParams{
		Name:      name,
		NetworkID: networkID,
	})
	if err != nil {
		return nil, err
	}
	// Fetch network info for the new chaincode
	net, err := s.queries.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, err
	}
	return &Chaincode{
		ID:              cc.ID,
		Name:            cc.Name,
		NetworkID:       cc.NetworkID,
		NetworkName:     net.Name,
		NetworkPlatform: net.Platform,
		CreatedAt:       nullTimeToString(cc.CreatedAt),
	}, nil
}

func (s *ChaincodeService) ListChaincodes(ctx context.Context) ([]*Chaincode, error) {
	dbChaincodes, err := s.queries.ListChaincodes(ctx)
	if err != nil {
		return nil, err
	}
	var result []*Chaincode
	for _, cc := range dbChaincodes {
		// Fetch network info for each chaincode
		net, err := s.queries.GetNetwork(ctx, cc.NetworkID)
		if err != nil {
			return nil, err
		}
		result = append(result, &Chaincode{
			ID:              cc.ID,
			Name:            cc.Name,
			NetworkID:       cc.NetworkID,
			NetworkName:     net.Name,
			NetworkPlatform: net.Platform,
			CreatedAt:       nullTimeToString(cc.CreatedAt),
		})
	}
	return result, nil
}

func (s *ChaincodeService) GetChaincode(ctx context.Context, id int64) (*Chaincode, error) {
	cc, err := s.queries.GetChaincode(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Chaincode{
		ID:              cc.ID,
		Name:            cc.Name,
		NetworkID:       cc.NetworkID,
		NetworkName:     cc.NetworkName,
		NetworkPlatform: cc.NetworkPlatform,
		CreatedAt:       nullTimeToString(cc.CreatedAt),
	}, nil
}

func (s *ChaincodeService) UpdateChaincode(ctx context.Context, id int64, name string, networkID int64) (*Chaincode, error) {
	cc, err := s.queries.UpdateChaincode(ctx, &db.UpdateChaincodeParams{
		ID:        id,
		Name:      name,
		NetworkID: networkID,
	})
	if err != nil {
		return nil, err
	}
	return &Chaincode{
		ID:        cc.ID,
		Name:      cc.Name,
		NetworkID: cc.NetworkID,
		CreatedAt: nullTimeToString(cc.CreatedAt),
	}, nil
}

func (s *ChaincodeService) DeleteChaincode(ctx context.Context, id int64) error {
	return s.queries.DeleteChaincode(ctx, id)
}

// --- ChaincodeDefinition CRUD ---
func (s *ChaincodeService) CreateChaincodeDefinition(ctx context.Context, chaincodeID int64, version string, sequence int64, dockerImage, endorsementPolicy, chaincodeAddress string) (*ChaincodeDefinition, error) {
	def, err := s.queries.CreateChaincodeDefinition(ctx, &db.CreateChaincodeDefinitionParams{
		ChaincodeID:       chaincodeID,
		Version:           version,
		Sequence:          sequence,
		DockerImage:       dockerImage,
		EndorsementPolicy: sql.NullString{String: endorsementPolicy, Valid: endorsementPolicy != ""},
		ChaincodeAddress:  sql.NullString{String: chaincodeAddress, Valid: chaincodeAddress != ""},
	})
	if err != nil {
		return nil, err
	}
	return &ChaincodeDefinition{
		ID:                def.ID,
		ChaincodeID:       def.ChaincodeID,
		Version:           def.Version,
		Sequence:          def.Sequence,
		DockerImage:       def.DockerImage,
		EndorsementPolicy: nullStringToString(def.EndorsementPolicy),
		ChaincodeAddress:  nullStringToString(def.ChaincodeAddress),
		CreatedAt:         nullTimeToString(def.CreatedAt),
	}, nil
}

// Helper to convert db.FabricChaincodeDefinition to ChaincodeDefinition (without DockerInfo)
func dbDefToSvcDef(def db.FabricChaincodeDefinition) *ChaincodeDefinition {
	return &ChaincodeDefinition{
		ID:                def.ID,
		ChaincodeID:       def.ChaincodeID,
		Version:           def.Version,
		Sequence:          def.Sequence,
		DockerImage:       def.DockerImage,
		EndorsementPolicy: nullStringToString(def.EndorsementPolicy),
		CreatedAt:         nullTimeToString(def.CreatedAt),
		ChaincodeAddress:  nullStringToString(def.ChaincodeAddress),
	}
}

func (s *ChaincodeService) ListChaincodeDefinitions(ctx context.Context, chaincodeID int64) ([]*ChaincodeDefinition, error) {
	defs, err := s.queries.ListChaincodeDefinitions(ctx, chaincodeID)
	if err != nil {
		return nil, err
	}

	// Create a slice to hold the results
	result := make([]*ChaincodeDefinition, len(defs))

	// Process each definition concurrently
	for i, def := range defs {
		defSvc := dbDefToSvcDef(*def)
		result[i] = defSvc
	}

	return result, nil
}

func (s *ChaincodeService) GetChaincodeDefinition(ctx context.Context, id int64) (*ChaincodeDefinition, error) {
	def, err := s.queries.GetChaincodeDefinition(ctx, id)
	if err != nil {
		return nil, err
	}
	defSvc := dbDefToSvcDef(*def)
	return defSvc, nil
}

func (s *ChaincodeService) UpdateChaincodeDefinition(ctx context.Context, id int64, version string, sequence int64, dockerImage, endorsementPolicy, chaincodeAddress string) (*ChaincodeDefinition, error) {
	def, err := s.queries.UpdateChaincodeDefinition(ctx, &db.UpdateChaincodeDefinitionParams{
		ID:                id,
		Version:           version,
		Sequence:          sequence,
		DockerImage:       dockerImage,
		EndorsementPolicy: sql.NullString{String: endorsementPolicy, Valid: endorsementPolicy != ""},
		ChaincodeAddress:  sql.NullString{String: chaincodeAddress, Valid: chaincodeAddress != ""},
	})
	if err != nil {
		return nil, err
	}
	return &ChaincodeDefinition{
		ID:                def.ID,
		ChaincodeID:       def.ChaincodeID,
		Version:           def.Version,
		Sequence:          def.Sequence,
		DockerImage:       def.DockerImage,
		EndorsementPolicy: nullStringToString(def.EndorsementPolicy),
		ChaincodeAddress:  nullStringToString(def.ChaincodeAddress),
		CreatedAt:         nullTimeToString(def.CreatedAt),
	}, nil
}

func (s *ChaincodeService) DeleteChaincodeDefinition(ctx context.Context, id int64) error {
	return s.queries.DeleteChaincodeDefinition(ctx, id)
}

// --- PeerStatus operations ---
func (s *ChaincodeService) SetPeerStatus(ctx context.Context, definitionID, peerID int64, status string) (*PeerStatus, error) {
	ps, err := s.queries.SetPeerStatus(ctx, &db.SetPeerStatusParams{
		DefinitionID: definitionID,
		PeerID:       peerID,
		Status:       status,
	})
	if err != nil {
		return nil, err
	}
	return &PeerStatus{
		ID:           ps.ID,
		DefinitionID: ps.DefinitionID,
		PeerID:       ps.PeerID,
		Status:       ps.Status,
		LastUpdated:  nullTimeToString(ps.LastUpdated),
	}, nil
}

// --- Utility functions for sql.NullTime and sql.NullString ---
func nullTimeToString(nt sql.NullTime) string {
	if nt.Valid {
		return nt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	return ""
}

func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// --- Docker utility functions (as before, using unified types from fabric.go) ---
// (Assume DockerContainerInfo and FabricChaincodeDetail are imported from fabric.go)

// DockerContainerInfo holds Docker container metadata for a chaincode
// Exported for use in HTTP and service layers
type DockerContainerInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	State        string   `json:"state"`
	Status       string   `json:"status"`
	DockerStatus string   `json:"docker_status"` // direct from Docker inspect
	Ports        []string `json:"ports"`
	Created      int64    `json:"created"`
}

// FabricChaincodeDetail provides a full view of a chaincode, its definitions, and Docker info (if deployed)
type FabricChaincodeDetail struct {
	Chaincode   *Chaincode             `json:"chaincode"`
	Definitions []*ChaincodeDefinition `json:"definitions"`
}

// GetChaincodeDetail returns a FabricChaincodeDetail for the given chaincode ID, including definitions and Docker info if deployed.
func (s *ChaincodeService) GetChaincodeDetail(ctx context.Context, id int64) (*FabricChaincodeDetail, error) {
	cc, err := s.GetChaincode(ctx, id)
	if err != nil {
		return nil, err
	}
	if cc == nil {
		return nil, nil
	}
	defs, err := s.ListChaincodeDefinitions(ctx, id)
	if err != nil {
		return nil, err
	}
	// Remove old DockerInfo field from FabricChaincodeDetail (now per-definition)
	return &FabricChaincodeDetail{
		Chaincode:   cc,
		Definitions: defs,
	}, nil
}

// getDockerInfoForDefinition returns DockerContainerInfo for a chaincode definition if deployed, or nil if not found
func getDockerInfoForDefinition(ctx context.Context, def *ChaincodeDefinition) (*DockerContainerInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	label := fmt.Sprintf("chainlaunch.chaincode.definition_id=%d", def.ID)
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", label)
	containers, err := cli.ContainerList(ctx, container.ListOptions{Filters: filterArgs})
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, nil
	}
	c := containers[0]
	ports := []string{}
	for _, p := range c.Ports {
		ports = append(ports, fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
	}
	inspect, err := cli.ContainerInspect(ctx, c.ID)
	dockerStatus := ""
	if err == nil {
		dockerStatus = inspect.State.Status
		if inspect.State.Health != nil {
			dockerStatus += ", health: " + inspect.State.Health.Status
		}
	}
	result := &DockerContainerInfo{
		ID:           c.ID,
		Name:         c.Names[0],
		Image:        c.Image,
		State:        c.State,
		Status:       c.Status,
		DockerStatus: dockerStatus,
		Ports:        ports,
		Created:      c.Created,
	}
	return result, nil
}

// Event data structs for chaincode definition events
type InstallChaincodeEventData struct {
	PeerIDs      []int64 `json:"peer_ids"`
	Result       string  `json:"result,omitempty"`
	ErrorMessage string  `json:"error_message,omitempty"`
}

type ApproveChaincodeEventData struct {
	PeerID       int64  `json:"peer_id"`
	Result       string `json:"result,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type CommitChaincodeEventData struct {
	PeerID       int64  `json:"peer_id"`
	Result       string `json:"result,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type DeployChaincodeEventData struct {
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Result        string `json:"result,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// InstallChaincodeByDefinition installs a chaincode definition on the given peers
func (s *ChaincodeService) InstallChaincodeByDefinition(ctx context.Context, definitionID int64, peerIDs []int64) error {
	definition, err := s.GetChaincodeDefinition(ctx, definitionID)
	if err != nil {
		eventData := InstallChaincodeEventData{PeerIDs: peerIDs, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "install", eventData)
		return err
	}
	chaincode, err := s.GetChaincode(ctx, definition.ChaincodeID)
	if err != nil {
		eventData := InstallChaincodeEventData{PeerIDs: peerIDs, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "install", eventData)
		return err
	}
	chaincodeAddress, _, err := s.ensureChaincodeAddress(ctx, definition)
	if err != nil {
		eventData := InstallChaincodeEventData{PeerIDs: peerIDs, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "install", eventData)
		return err
	}
	label := chaincode.Name
	codeTarGz, err := s.getCodeTarGz(chaincodeAddress, "", "", "", "")
	if err != nil {
		eventData := InstallChaincodeEventData{PeerIDs: peerIDs, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "install", eventData)
		return err
	}
	pkg, err := s.getChaincodePackage(label, codeTarGz)
	if err != nil {
		eventData := InstallChaincodeEventData{PeerIDs: peerIDs, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "install", eventData)
		return err
	}
	var lastErr error
	for _, peerID := range peerIDs {
		peerService, peerConn, err := s.nodeService.GetFabricPeerService(ctx, peerID)
		if err != nil {
			lastErr = err
			continue
		}
		defer peerConn.Close()
		_, err = peerService.Install(ctx, bytes.NewReader(pkg))
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		eventData := InstallChaincodeEventData{PeerIDs: peerIDs, Result: "failure", ErrorMessage: lastErr.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "install", eventData)
		return lastErr
	}
	eventData := InstallChaincodeEventData{PeerIDs: peerIDs, Result: "success"}
	_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "install", eventData)
	return nil
}

func (s *ChaincodeService) getCodeTarGz(
	address string,
	rootCert string,
	clientKey string,
	clientCert string,
	metaInfPath string,
) ([]byte, error) {
	var err error
	// Determine if TLS is required based on certificate presence
	tlsRequired := rootCert != ""
	clientAuthRequired := clientCert != "" && clientKey != ""

	// Read certificate files if provided
	var rootCertContent, clientKeyContent, clientCertContent string
	if tlsRequired {
		rootCertBytes, err := os.ReadFile(rootCert)
		if err != nil {
			return nil, fmt.Errorf("failed to read root certificate: %w", err)
		}
		rootCertContent = string(rootCertBytes)
	}

	if clientAuthRequired {
		clientKeyBytes, err := os.ReadFile(clientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to read client key: %w", err)
		}
		clientKeyContent = string(clientKeyBytes)

		clientCertBytes, err := os.ReadFile(clientCert)
		if err != nil {
			return nil, fmt.Errorf("failed to read client certificate: %w", err)
		}
		clientCertContent = string(clientCertBytes)
	}

	connMap := map[string]interface{}{
		"address":              address,
		"dial_timeout":         "10s",
		"tls_required":         tlsRequired,
		"root_cert":            rootCertContent,
		"client_auth_required": clientAuthRequired,
		"client_key":           clientKeyContent,
		"client_cert":          clientCertContent,
	}
	connJsonBytes, err := json.Marshal(connMap)
	if err != nil {
		return nil, err
	}
	s.logger.Debugf("Conn=%s", string(connJsonBytes))
	// set up the output file
	buf := &bytes.Buffer{}
	// set up the gzip writer
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	header := new(tar.Header)
	header.Name = "connection.json"
	header.Size = int64(len(connJsonBytes))
	header.Mode = 0755
	err = tw.WriteHeader(header)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(connJsonBytes)
	_, err = io.Copy(tw, r)
	if err != nil {
		return nil, err
	}
	if metaInfPath != "" {
		src := metaInfPath
		// walk through 3 file in the folder
		err = filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
			// generate tar header
			header, err := tar.FileInfoHeader(fi, file)
			if err != nil {
				return err
			}

			// must provide real name
			// (see https://golang.org/src/archive/tar/common.go?#L626)
			relname, err := filepath.Rel(src, file)
			if err != nil {
				return err
			}
			if relname == "." {
				return nil
			}
			header.Name = "META-INF/" + filepath.ToSlash(relname)

			// write header
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			// if not a dir, write file content
			if !fi.IsDir() {
				data, err := os.Open(file)
				if err != nil {
					return err
				}
				if _, err := io.Copy(tw, data); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	err = tw.Close()
	if err != nil {
		return nil, err
	}
	err = gw.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *ChaincodeService) getChaincodePackage(label string, codeTarGz []byte) ([]byte, error) {
	var err error
	metadataJson := fmt.Sprintf(`
{
  "type": "ccaas",
  "label": "%s"
}
`, label)
	// set up the output file
	buf := &bytes.Buffer{}

	// set up the gzip writer
	gw := gzip.NewWriter(buf)
	defer func(gw *gzip.Writer) {
		err := gw.Close()
		if err != nil {
			s.logger.Warnf("gzip.Writer.Close() failed: %s", err)
		}
	}(gw)
	tw := tar.NewWriter(gw)
	defer func(tw *tar.Writer) {
		err := tw.Close()
		if err != nil {
			s.logger.Warnf("tar.Writer.Close() failed: %s", err)
		}
	}(tw)
	header := new(tar.Header)
	header.Name = "metadata.json"
	metadataJsonBytes := []byte(metadataJson)
	header.Size = int64(len(metadataJsonBytes))
	header.Mode = 0777
	err = tw.WriteHeader(header)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(metadataJsonBytes)
	_, err = io.Copy(tw, r)
	if err != nil {
		return nil, err
	}
	headerCode := new(tar.Header)
	headerCode.Name = "code.tar.gz"
	headerCode.Size = int64(len(codeTarGz))
	headerCode.Mode = 0777
	err = tw.WriteHeader(headerCode)
	if err != nil {
		return nil, err
	}
	r = bytes.NewReader(codeTarGz)
	_, err = io.Copy(tw, r)
	if err != nil {
		return nil, err
	}
	err = tw.Close()
	if err != nil {
		return nil, err
	}
	err = gw.Close()
	if err != nil {
		s.logger.Warnf("gzip.Writer.Close() failed: %s", err)
		return nil, err
	}
	return buf.Bytes(), nil
}

// ApproveChaincodeByDefinition approves a chaincode definition using the given peer
func (s *ChaincodeService) ApproveChaincodeByDefinition(ctx context.Context, definitionID int64, peerID int64) error {
	peerGateway, peerConn, err := s.nodeService.GetFabricPeerGateway(ctx, peerID)
	if err != nil {
		eventData := ApproveChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "approve", eventData)
		return err
	}
	defer peerConn.Close()
	definition, err := s.GetChaincodeDefinition(ctx, definitionID)
	if err != nil {
		eventData := ApproveChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "approve", eventData)
		return err
	}
	chaincodeDef, err := s.buildChaincodeDefinition(ctx, definition)
	if err != nil {
		eventData := ApproveChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "approve", eventData)
		return err
	}
	if chaincodeDef.PackageID == "" {
		eventData := ApproveChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: "package ID is empty"}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "approve", eventData)
		return fmt.Errorf("package ID is empty")
	}
	err = peerGateway.Approve(ctx, chaincodeDef)
	if err != nil {
		endorseError, ok := err.(*fabricclient.EndorseError)
		if ok {
			detailsStr := []string{}
			for _, detail := range status.Convert(err).Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s\n", detail.GetAddress(), detail.GetMspId(), detail.GetMessage()))
				}
			}
			err = fmt.Errorf("failed to approve chaincode: %s (gRPC status: %s)",
				endorseError.TransactionError.Error(),
				strings.Join(detailsStr, "\n"))
		} else {
			err = fmt.Errorf("failed to approve chaincode: %w", err)
		}
		eventData := ApproveChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "approve", eventData)
		return err
	}
	eventData := ApproveChaincodeEventData{PeerID: peerID, Result: "success"}
	_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "approve", eventData)
	return nil
}

// CommitChaincodeByDefinition commits a chaincode definition using the given peer
func (s *ChaincodeService) CommitChaincodeByDefinition(ctx context.Context, definitionID int64, peerID int64) error {
	peerGateway, peerConn, err := s.nodeService.GetFabricPeerGateway(ctx, peerID)
	if err != nil {
		eventData := CommitChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "commit", eventData)
		return err
	}
	defer peerConn.Close()
	definition, err := s.GetChaincodeDefinition(ctx, definitionID)
	if err != nil {
		eventData := CommitChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "commit", eventData)
		return err
	}
	chaincodeDef, err := s.buildChaincodeDefinition(ctx, definition)
	if err != nil {
		eventData := CommitChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "commit", eventData)
		return err
	}
	err = peerGateway.Commit(ctx, chaincodeDef)
	if err != nil {
		endorseError, ok := err.(*fabricclient.EndorseError)
		if ok {
			detailsStr := []string{}
			for _, detail := range status.Convert(err).Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s\n", detail.GetAddress(), detail.GetMspId(), detail.GetMessage()))
				}
			}
			err = fmt.Errorf("failed to approve chaincode: %s (gRPC status: %s)",
				endorseError.TransactionError.Error(),
				strings.Join(detailsStr, "\n"))
		} else {
			err = fmt.Errorf("failed to approve chaincode: %w", err)
		}
		eventData := ApproveChaincodeEventData{PeerID: peerID, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "approve", eventData)
		return err
	}
	eventData := CommitChaincodeEventData{PeerID: peerID, Result: "success"}
	_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "commit", eventData)
	return nil
}

// buildChaincodeDefinition builds a chaincode.Definition from a ChaincodeDefinition
func (s *ChaincodeService) buildChaincodeDefinition(ctx context.Context, definition *ChaincodeDefinition) (*chaincode.Definition, error) {
	chaincodeDB, err := s.GetChaincode(ctx, definition.ChaincodeID)
	if err != nil {
		return nil, err
	}
	networkDB, err := s.queries.GetNetwork(ctx, chaincodeDB.NetworkID)
	if err != nil {
		return nil, err
	}
	applicationPolicy, err := chaincode.NewApplicationPolicy(definition.EndorsementPolicy, "")
	if err != nil {
		return nil, err
	}
	packageID, _, err := s.getChaincodePackageInfo(chaincodeDB, definition)
	if err != nil {
		return nil, err
	}
	chaincodeDef := &chaincode.Definition{
		Name:              chaincodeDB.Name,
		Version:           definition.Version,
		Sequence:          definition.Sequence,
		ChannelName:       networkDB.Name,
		ApplicationPolicy: applicationPolicy,
		InitRequired:      false,
		Collections:       nil,
		PackageID:         packageID,
		EndorsementPlugin: "escc",
		ValidationPlugin:  "vscc",
	}
	return chaincodeDef, nil
}

// getChaincodePackageInfo returns the package ID and chaincode package bytes for a given chaincode and definition
func (s *ChaincodeService) getChaincodePackageInfo(chaincode *Chaincode, definition *ChaincodeDefinition) (string, []byte, error) {
	label := chaincode.Name
	codeTarGz, err := s.getCodeTarGz(definition.ChaincodeAddress, "", "", "", "")
	if err != nil {
		return "", nil, err
	}
	pkg, err := s.getChaincodePackage(label, codeTarGz)
	if err != nil {
		return "", nil, err
	}
	packageID := GetPackageID(label, pkg)
	return packageID, pkg, nil
}

// GetPackageID returns the package ID with the label and hash of the chaincode install package
func GetPackageID(label string, ccInstallPkg []byte) string {
	h := sha256.New()
	h.Write(ccInstallPkg)
	hash := h.Sum(nil)
	return fmt.Sprintf("%s:%x", label, hash)
}

// buildChaincodeDockerLabels builds a map of Docker labels for chaincode deployment
func buildChaincodeDockerLabels(definition *ChaincodeDefinition, chaincode *Chaincode) map[string]string {
	return map[string]string{
		"chainlaunch.chaincode.definition_id": fmt.Sprintf("%d", definition.ID),
		"chainlaunch.chaincode.name":          chaincode.Name,
		"chainlaunch.chaincode.version":       definition.Version,
		"chainlaunch.chaincode.sequence":      fmt.Sprintf("%d", definition.Sequence),
		"chainlaunch.chaincode.network_id":    fmt.Sprintf("%d", chaincode.NetworkID),
		"chainlaunch.chaincode.network_name":  chaincode.NetworkName,
		"chainlaunch.chaincode.address":       definition.ChaincodeAddress,
	}
}

// DeployChaincodeByDefinition deploys a chaincode definition using Docker image
func (s *ChaincodeService) DeployChaincodeByDefinition(ctx context.Context, definitionID int64) error {
	definition, err := s.GetChaincodeDefinition(ctx, definitionID)
	if err != nil {
		eventData := DeployChaincodeEventData{HostPort: "", ContainerPort: "", Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "deploy", eventData)
		return err
	}
	internalPort := "7052"
	host, portStr, err := net.SplitHostPort(definition.ChaincodeAddress)
	if err != nil {
		eventData := DeployChaincodeEventData{HostPort: "", ContainerPort: "", Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "deploy", eventData)
		return err
	}
	exposedPort, err := strconv.Atoi(portStr)
	if err != nil {
		eventData := DeployChaincodeEventData{HostPort: "", ContainerPort: "", Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "deploy", eventData)
		return err
	}
	chaincodeDB, err := s.GetChaincode(ctx, definition.ChaincodeID)
	if err != nil {
		eventData := DeployChaincodeEventData{HostPort: fmt.Sprintf("%s:%d", host, exposedPort), ContainerPort: internalPort, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "deploy", eventData)
		return err
	}
	packageID, _, err := s.getChaincodePackageInfo(chaincodeDB, definition)
	if err != nil {
		eventData := DeployChaincodeEventData{HostPort: fmt.Sprintf("%s:%d", host, exposedPort), ContainerPort: internalPort, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "deploy", eventData)
		return err
	}
	labels := buildChaincodeDockerLabels(definition, chaincodeDB)
	reporter := &loggerStatusReporter{logger: s.logger}
	_, err = DeployChaincodeWithDockerImageWithLabels(definition.DockerImage, packageID, portStr, internalPort, labels, reporter)
	if err != nil {
		eventData := DeployChaincodeEventData{HostPort: fmt.Sprintf("%s:%d", host, exposedPort), ContainerPort: internalPort, Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "deploy", eventData)
		return err
	}
	eventData := DeployChaincodeEventData{HostPort: fmt.Sprintf("%s:%d", host, exposedPort), ContainerPort: internalPort, Result: "success", ErrorMessage: ""}
	_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "deploy", eventData)
	// Optionally: store the labels somewhere if you want to track them outside Docker
	return nil
}

// loggerStatusReporter implements DeploymentStatusReporter using the service logger
// Used for reporting status in DeployChaincodeByDefinition
// Not exported
type loggerStatusReporter struct {
	logger *logger.Logger
}

func (r *loggerStatusReporter) ReportStatus(update DeploymentStatusUpdate) {
	if update.Error != nil {
		r.logger.Errorf("[DeployStatus] %s: %s (error: %v)", update.Status, update.Message, update.Error)
	} else {
		r.logger.Infof("[DeployStatus] %s: %s", update.Status, update.Message)
	}
}

// GetStatus is a no-op for loggerStatusReporter (returns zero value)
func (r *loggerStatusReporter) GetStatus(deploymentID string) DeploymentStatusUpdate {
	return DeploymentStatusUpdate{}
}

// ChaincodeDefinitionEvent represents a timeline event for a chaincode definition
type ChaincodeDefinitionEvent struct {
	ID           int64       `json:"id"`
	DefinitionID int64       `json:"definition_id"`
	EventType    string      `json:"event_type"`
	EventData    interface{} `json:"event_data"`
	CreatedAt    string      `json:"created_at"`
}

// AddChaincodeDefinitionEvent logs an event for a chaincode definition
func (s *ChaincodeService) AddChaincodeDefinitionEvent(ctx context.Context, definitionID int64, eventType string, eventData interface{}) error {
	dataBytes, err := json.Marshal(eventData)
	if err != nil {
		return err
	}
	return s.queries.AddChaincodeDefinitionEvent(ctx, &db.AddChaincodeDefinitionEventParams{
		DefinitionID: definitionID,
		EventType:    eventType,
		EventData:    sql.NullString{String: string(dataBytes), Valid: true},
	})
}

// ListChaincodeDefinitionEvents returns the timeline of events for a chaincode definition
func (s *ChaincodeService) ListChaincodeDefinitionEvents(ctx context.Context, definitionID int64) ([]*ChaincodeDefinitionEvent, error) {
	dbEvents, err := s.queries.ListChaincodeDefinitionEvents(ctx, definitionID)
	if err != nil {
		return nil, err
	}
	var events []*ChaincodeDefinitionEvent
	for _, e := range dbEvents {
		var eventData interface{}
		if e.EventData.String != "" {
			_ = json.Unmarshal([]byte(e.EventData.String), &eventData)
		}
		events = append(events, &ChaincodeDefinitionEvent{
			ID:           e.ID,
			DefinitionID: e.DefinitionID,
			EventType:    e.EventType,
			EventData:    eventData,
			CreatedAt:    nullTimeToString(e.CreatedAt),
		})
	}
	return events, nil
}

// ensureChaincodeAddress ensures the chaincode address is set and available, updating the DB if needed.
func (s *ChaincodeService) ensureChaincodeAddress(ctx context.Context, definition *ChaincodeDefinition) (string, string, error) {
	externalIP := "0.0.0.0"
	chaincodeAddress := definition.ChaincodeAddress
	exposedPort := ""
	if chaincodeAddress == "" {
		alloc, err := ports.GetFreePort("fabric-chaincode")
		if err != nil {
			return "", "", fmt.Errorf("no free ports available for chaincode: %w", err)
		}
		exposedPort = fmt.Sprintf("%d", alloc.Port)
		chaincodeAddress = fmt.Sprintf("%s:%s", externalIP, exposedPort)
		err = s.queries.UpdateFabricChaincodeDefinitionAddress(ctx, &db.UpdateFabricChaincodeDefinitionAddressParams{
			ChaincodeAddress: sql.NullString{String: chaincodeAddress, Valid: true},
			ID:               definition.ID,
		})
		if err != nil {
			return "", "", fmt.Errorf("failed to update chaincode address in db: %w", err)
		}
	}
	return chaincodeAddress, exposedPort, nil
}

// RemoveDeploymentByDefinition removes the Docker deployment for a chaincode definition
func (s *ChaincodeService) RemoveDeploymentByDefinition(ctx context.Context, definitionID int64) error {
	definition, err := s.GetChaincodeDefinition(ctx, definitionID)
	if err != nil {
		eventData := DeployChaincodeEventData{HostPort: "", ContainerPort: "", Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "undeploy", eventData)
		return err
	}
	// chaincodeDB is not needed for removal
	deployer := NewDockerChaincodeDeployer()
	if deployer == nil {
		err := fmt.Errorf("failed to create DockerChaincodeDeployer")
		eventData := DeployChaincodeEventData{HostPort: "", ContainerPort: "", Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "undeploy", eventData)
		return err
	}
	// Remove containers by label
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("chainlaunch.chaincode.definition_id=%d", definitionID))
	containers, err := deployer.client.ContainerList(ctx, container.ListOptions{All: true, Filters: filterArgs})
	if err != nil {
		eventData := DeployChaincodeEventData{HostPort: "", ContainerPort: "", Result: "failure", ErrorMessage: err.Error()}
		_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "undeploy", eventData)
		return err
	}
	for _, c := range containers {
		if err := deployer.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
			eventData := DeployChaincodeEventData{HostPort: "", ContainerPort: "", Result: "failure", ErrorMessage: err.Error()}
			_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "undeploy", eventData)
			return err
		}
	}
	eventData := DeployChaincodeEventData{HostPort: definition.ChaincodeAddress, ContainerPort: "7052", Result: "success", ErrorMessage: ""}
	_ = s.AddChaincodeDefinitionEvent(ctx, definitionID, "undeploy", eventData)
	return nil
}

// InvokeChaincode submits a transaction to a chaincode
func (s *ChaincodeService) InvokeChaincode(ctx context.Context, chaincodeId int64, function string, args []string, channel string, transient map[string][]byte, keyID int64) (interface{}, error) {
	cc, err := s.GetChaincode(ctx, chaincodeId)
	if err != nil {
		return nil, err
	}
	if cc == nil {
		return nil, fmt.Errorf("chaincode not found")
	}
	// Get MSP ID from key
	key, err := s.keyManagementService.GetKey(ctx, int(keyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	if key.Certificate == nil || *key.Certificate == "" {
		return nil, fmt.Errorf("key does not have a certificate")
	}
	cert, err := keymgmtservice.ParseCertificate(*key.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}
	if len(cert.Subject.Organization) == 0 {
		return nil, fmt.Errorf("certificate does not contain organization (MSP ID)")
	}
	mspID := cert.Subject.Organization[0]
	// Get all nodes and select a peer with matching MSP ID
	nodes, err := s.nodeService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}
	var matchingPeerIDs []int64
	for _, node := range nodes.Items {
		if node.NodeType == "FABRIC_PEER" && node.FabricPeer != nil && node.FabricPeer.MSPID == mspID && node.Platform == "FABRIC" {
			matchingPeerIDs = append(matchingPeerIDs, node.ID)
		}
	}
	if len(matchingPeerIDs) == 0 {
		return nil, fmt.Errorf("no peer found for MSP ID: %s", mspID)
	}
	// Pick a random peer if more than one
	rand.Seed(time.Now().UnixNano())
	peerID := matchingPeerIDs[rand.Intn(len(matchingPeerIDs))]
	// Use selected peer
	gateway, conn, err := s.nodeService.GetFabricPeerClientGateway(ctx, peerID, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer gateway: %w", err)
	}
	defer conn.Close()
	networkName := channel
	if networkName == "" {
		networkName = cc.NetworkName
	}
	network := gateway.GetNetwork(networkName)
	contract := network.GetContract(cc.Name)
	var result []byte
	var commit *fabricclient.Commit
	if transient != nil && len(transient) > 0 {
		result, commit, err = contract.SubmitAsync(function, fabricclient.WithArguments(args...), fabricclient.WithTransient(transient))
	} else {
		result, commit, err = contract.SubmitAsync(function, fabricclient.WithArguments(args...))
	}
	if err != nil {
		return nil, err
	}
	txStatus, err := commit.Status()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"result":        string(result),
		"blockNumber":   txStatus.BlockNumber,
		"transactionId": txStatus.TransactionID,
		"code":          txStatus.Code,
	}, nil
}

// QueryChaincode evaluates a transaction on a chaincode
func (s *ChaincodeService) QueryChaincode(ctx context.Context, chaincodeId int64, function string, args []string, channel string, transient map[string][]byte, keyID int64) (interface{}, error) {
	cc, err := s.GetChaincode(ctx, chaincodeId)
	if err != nil {
		return nil, err
	}
	if cc == nil {
		return nil, fmt.Errorf("chaincode not found")
	}
	// Get MSP ID from key
	key, err := s.keyManagementService.GetKey(ctx, int(keyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	if key.Certificate == nil || *key.Certificate == "" {
		return nil, fmt.Errorf("key does not have a certificate")
	}
	cert, err := keymgmtservice.ParseCertificate(*key.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}
	if len(cert.Subject.Organization) == 0 {
		return nil, fmt.Errorf("certificate does not contain organization (MSP ID)")
	}
	mspID := cert.Subject.Organization[0]
	// Get all nodes and select a peer with matching MSP ID
	nodes, err := s.nodeService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}
	var matchingPeerIDs []int64
	for _, node := range nodes.Items {
		if node.NodeType == "FABRIC_PEER" && node.FabricPeer != nil && node.FabricPeer.MSPID == mspID && node.Platform == "FABRIC" {
			matchingPeerIDs = append(matchingPeerIDs, node.ID)
		}
	}
	if len(matchingPeerIDs) == 0 {
		return nil, fmt.Errorf("no peer found for MSP ID: %s", mspID)
	}
	// Pick a random peer if more than one
	rand.Seed(time.Now().UnixNano())
	peerID := matchingPeerIDs[rand.Intn(len(matchingPeerIDs))]
	// Use selected peer
	gatewayFabric, conn, err := s.nodeService.GetFabricPeerClientGateway(ctx, peerID, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer gateway: %w", err)
	}
	defer conn.Close()
	networkName := channel
	if networkName == "" {
		networkName = cc.NetworkName
	}
	network := gatewayFabric.GetNetwork(networkName)
	contract := network.GetContract(cc.Name)
	result, err := contract.EvaluateTransaction(function, args...)
	if err != nil {
		endorseError, ok := err.(*fabricclient.EndorseError)
		if ok {
			detailsStr := []string{}
			for _, detail := range status.Convert(err).Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s\n", detail.GetAddress(), detail.GetMspId(), detail.GetMessage()))
				}
			}
			return nil, fmt.Errorf("failed to submit transaction: %s (gRPC status: %s)",
				endorseError.TransactionError.Error(),
				strings.Join(detailsStr, "\n"))
		}
		statusError := status.Convert(err)
		if statusError != nil {
			detailsStr := []string{}
			for _, detail := range statusError.Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s",
						detail.GetAddress(),
						detail.GetMspId(),
						detail.GetMessage()))
				}
			}
			return nil, fmt.Errorf("failed to submit transaction: %s (gRPC status details: %s)",
				statusError.Message(),
				strings.Join(detailsStr, "\n"))
		}
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	return string(result), nil
}
