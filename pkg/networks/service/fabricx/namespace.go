package fabricx

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"time"

	"github.com/chainlaunch/chainlaunch/internal/protoutil"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"

	adminidentity "github.com/hyperledger/fabric-admin-sdk/pkg/identity"
	gwidentity "github.com/hyperledger/fabric-gateway/pkg/identity"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	msppb_fabric "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/hyperledger/fabric-x-common/api/applicationpb"
	"github.com/hyperledger/fabric-x-common/api/committerpb"
	"github.com/hyperledger/fabric-x-common/api/msppb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

// NamespaceCreateOptions describes a namespace creation request.
type NamespaceCreateOptions struct {
	// Name is the string identifier written to _meta.
	Name string
	// Version is -1 for a new namespace, >=0 to update an existing one.
	Version int
	// SubmitterOrgID is the fabric_organizations.id whose MSP identity will sign
	// the transaction. Must be a channel member.
	SubmitterOrgID int64
	// WaitForFinality, when true, opens a Deliver stream on the submitter's
	// committer-sidecar and waits up to FinalityTimeout for the tx to commit.
	WaitForFinality bool
	// FinalityTimeout defaults to 2 minutes when zero.
	FinalityTimeout time.Duration
	// EndorsementPublicKeyPEM is an optional PEM-encoded PKIX public key to
	// store as the namespace's endorsement policy. If empty, the submitter
	// org's admin cert pubkey is used.
	//
	// Security note: the caller is effectively choosing who can endorse future
	// transactions on this namespace. The submitter admin still signs the
	// create-tx envelope (channel membership requirement), but the stored
	// policy pubkey governs subsequent endorsement checks.
	EndorsementPublicKeyPEM []byte
}

// NamespaceResult is what gets returned to the caller of CreateNamespace.
type NamespaceResult struct {
	Row  *db.FabricxNamespace
	TxID string
}

// CreateNamespace broadcasts a namespace-creation transaction to one of the
// network's orderer group routers and persists the attempt in the DB.
func (d *FabricXDeployer) CreateNamespace(ctx context.Context, networkID int64, opts NamespaceCreateOptions) (*NamespaceResult, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("namespace name is required")
	}
	if opts.SubmitterOrgID == 0 {
		return nil, fmt.Errorf("submitter_org_id is required")
	}
	if opts.FinalityTimeout == 0 {
		opts.FinalityTimeout = 2 * time.Minute
	}
	if opts.Version == 0 {
		// Keep the "0 = new namespace" ergonomics out of the proto path —
		// users usually want -1 for "create new". Treat 0 as new as well.
		opts.Version = -1
	}

	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network %d: %w", networkID, err)
	}
	if network.Platform != string(types.NetworkTypeFabricX) && network.Platform != "FABRICX" {
		return nil, fmt.Errorf("network %d is not a FabricX network (platform=%s)", networkID, network.Platform)
	}
	if !network.Config.Valid {
		return nil, fmt.Errorf("network %d has no config", networkID)
	}
	var cfg types.FabricXNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse network config: %w", err)
	}

	// Resolve submitter org + its MSP identity.
	org, err := d.orgService.GetOrganization(ctx, opts.SubmitterOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get submitter org %d: %w", opts.SubmitterOrgID, err)
	}
	if !org.AdminSignKeyID.Valid {
		return nil, fmt.Errorf("submitter org %s has no admin sign key", org.MspID)
	}
	adminKey, err := d.keyMgmt.GetKey(ctx, int(org.AdminSignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get admin sign key: %w", err)
	}
	if adminKey.Certificate == nil {
		return nil, fmt.Errorf("admin sign key has no certificate")
	}
	adminPriv, err := d.keyMgmt.GetDecryptedPrivateKey(int(org.AdminSignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt admin sign key: %w", err)
	}
	certPEM := []byte(*adminKey.Certificate)
	cert, err := gwidentity.CertificateFromPEM(certPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid admin sign certificate: %w", err)
	}
	privKey, err := gwidentity.PrivateKeyFromPEM([]byte(adminPriv))
	if err != nil {
		return nil, fmt.Errorf("invalid admin sign private key: %w", err)
	}
	signer, err := adminidentity.NewPrivateKeySigningIdentity(org.MspID, cert, privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to build signing identity: %w", err)
	}

	// Extract the public key (in PKIX DER) that becomes the endorsement policy
	// for the new namespace.
	pubDER, err := marshalPublicKey(cert)
	if err != nil {
		return nil, fmt.Errorf("failed to extract public key: %w", err)
	}

	// Allow callers to override the stored policy pubkey without changing the
	// tx signing identity. Typical use-case: a token-sdk-x FSC endorser that
	// signs token txs with a peer cert rather than the org admin cert.
	if len(opts.EndorsementPublicKeyPEM) > 0 {
		overrideDER, derErr := decodePEMPublicKeyToPKIXPEM(opts.EndorsementPublicKeyPEM)
		if derErr != nil {
			return nil, fmt.Errorf("invalid endorsement public key: %w", derErr)
		}
		pubDER = overrideDER
		d.logger.Info("Using override endorsement pubkey for namespace",
			"network", networkID, "name", opts.Name)
	}

	// Pick an orderer router to broadcast to. We also grab its TLS CA + external
	// endpoint so we can dial securely.
	ordererAddr, ordererTLSCA, err := d.pickOrdererRouter(ctx, &cfg, opts.SubmitterOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to pick orderer: %w", err)
	}

	// Record the attempt up-front so we have a row to update whether it
	// succeeds or fails.
	row, err := d.db.CreateFabricXNamespace(ctx, &db.CreateFabricXNamespaceParams{
		NetworkID:      networkID,
		Name:           opts.Name,
		Version:        int64(opts.Version),
		SubmitterMspID: org.MspID,
		SubmitterOrgID: sql.NullInt64{Int64: opts.SubmitterOrgID, Valid: true},
		TxID:           sql.NullString{},
		Status:         "pending",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record namespace row: %w", err)
	}

	markFailed := func(dberr error) error {
		_, _ = d.db.UpdateFabricXNamespaceStatus(ctx, &db.UpdateFabricXNamespaceStatusParams{
			ID:     row.ID,
			Status: "failed",
			TxID:   row.TxID,
			Error:  sql.NullString{String: dberr.Error(), Valid: true},
		})
		return dberr
	}

	// Build the namespace transaction and sign it.
	tx := buildNamespaceTx("ECDSA", pubDER, opts.Name, opts.Version)
	env, txID, err := signNamespaceEnvelope(signer, cfg.ChannelName, tx)
	if err != nil {
		return nil, markFailed(fmt.Errorf("failed to sign namespace tx: %w", err))
	}

	// Pre-register with the sidecar notifier so we don't miss the tx status
	// notification (the sidecar only surfaces events for TxIDs we've already
	// subscribed to). Done before broadcast to avoid racing the committer.
	var waiter *txStatusWaiter
	if opts.WaitForFinality {
		committerAddr, _, ferr := d.pickCommitterSidecar(ctx, &cfg, opts.SubmitterOrgID)
		if ferr != nil {
			d.logger.Warn("Skipping finality wait: no committer sidecar available", "err", ferr)
		} else {
			w, werr := openTxStatusWaiter(ctx, committerAddr, txID, opts.FinalityTimeout)
			if werr != nil {
				return nil, markFailed(fmt.Errorf("finality setup failed: %w", werr))
			}
			waiter = w
			defer waiter.Close()
		}
	}

	// Broadcast to the orderer router.
	if err := broadcastEnvelope(ctx, ordererAddr, ordererTLSCA, env); err != nil {
		return nil, markFailed(fmt.Errorf("broadcast failed: %w", err))
	}
	d.logger.Info("Namespace tx broadcast accepted", "network", networkID, "name", opts.Name, "txID", txID)

	status := "broadcast"
	if waiter != nil {
		if err := waiter.WaitForCommit(); err != nil {
			return nil, markFailed(fmt.Errorf("finality wait failed: %w", err))
		}
		status = "committed"
		d.logger.Info("Namespace tx committed", "network", networkID, "name", opts.Name, "txID", txID)
	}

	updated, err := d.db.UpdateFabricXNamespaceStatus(ctx, &db.UpdateFabricXNamespaceStatusParams{
		ID:     row.ID,
		Status: status,
		TxID:   sql.NullString{String: txID, Valid: true},
		Error:  sql.NullString{},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update namespace row: %w", err)
	}
	return &NamespaceResult{Row: updated, TxID: txID}, nil
}

// ListNamespaces returns all namespace rows recorded for this network.
func (d *FabricXDeployer) ListNamespaces(ctx context.Context, networkID int64) ([]*db.FabricxNamespace, error) {
	return d.db.ListFabricXNamespacesByNetwork(ctx, networkID)
}

// NamespaceView is the merged view of a namespace across the chain (source of
// truth via the committer query-service) and the local DB (tracks submission
// history: submitter, txID, PENDING/FAILED rows that never committed).
//
// Source is "chain" when present only on-chain (e.g. created outside
// ChainLaunch), "db" when present only locally (PENDING/FAILED submissions),
// "both" when both sides agree on the name.
type NamespaceView struct {
	Name    string
	Version uint64
	OnChain bool
	Source  string

	// Fields below are populated from the local DB row when one exists.
	ID             *int64
	NetworkID      *int64
	SubmitterMspID string
	SubmitterOrgID *int64
	TxID           *string
	Status         string
	Error          *string
	CreatedAt      *time.Time
	UpdatedAt      *time.Time
}

// ListNamespacesMerged returns the merged chain+DB view of namespaces in a
// FabricX network. The chain call is best-effort: if the committer is
// unreachable, the function still returns DB rows with OnChain=false and
// a non-nil chainErr so callers can surface the degraded state.
func (d *FabricXDeployer) ListNamespacesMerged(ctx context.Context, networkID int64) ([]NamespaceView, error, error) {
	rows, err := d.db.ListFabricXNamespacesByNetwork(ctx, networkID)
	if err != nil {
		return nil, nil, fmt.Errorf("list db namespace rows: %w", err)
	}

	policies, chainErr := d.GetNamespacePolicies(ctx, networkID)

	byName := make(map[string]*NamespaceView, len(rows)+len(policies))
	order := make([]string, 0, len(rows)+len(policies))

	for _, p := range policies {
		v := &NamespaceView{
			Name:    p.Namespace,
			Version: p.Version,
			OnChain: true,
			Source:  "chain",
			Status:  "COMMITTED",
		}
		byName[p.Namespace] = v
		order = append(order, p.Namespace)
	}

	for _, row := range rows {
		v, exists := byName[row.Name]
		if !exists {
			v = &NamespaceView{
				Name:   row.Name,
				Source: "db",
			}
			byName[row.Name] = v
			order = append(order, row.Name)
		} else {
			v.Source = "both"
		}
		id := row.ID
		nid := row.NetworkID
		v.ID = &id
		v.NetworkID = &nid
		v.SubmitterMspID = row.SubmitterMspID
		if row.SubmitterOrgID.Valid {
			x := row.SubmitterOrgID.Int64
			v.SubmitterOrgID = &x
		}
		if row.TxID.Valid {
			x := row.TxID.String
			v.TxID = &x
		}
		// DB status only overrides when the namespace is not on-chain. A
		// committed namespace with a stale PENDING/FAILED local row should
		// still report COMMITTED — the chain is authoritative.
		if !v.OnChain {
			v.Status = row.Status
			// DB version is only meaningful pre-commit; keep zero otherwise.
			if row.Version >= 0 {
				v.Version = uint64(row.Version)
			}
		}
		if row.Error.Valid {
			x := row.Error.String
			v.Error = &x
		}
		ct := row.CreatedAt
		v.CreatedAt = &ct
		if row.UpdatedAt.Valid {
			ut := row.UpdatedAt.Time
			v.UpdatedAt = &ut
		}
	}

	out := make([]NamespaceView, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, chainErr, nil
}

// DeleteNamespaceRecord removes a local record only. On-chain the namespace
// still exists (the chain model only supports updates, not deletion).
func (d *FabricXDeployer) DeleteNamespaceRecord(ctx context.Context, id int64) error {
	return d.db.DeleteFabricXNamespace(ctx, id)
}

// pickOrdererRouter returns a reachable router endpoint + TLS CA pem.
// Prefers the submitter org's own router so we exercise an identity-matched
// connection, falls back to the first orderer group in the config. Handles
// both the new node_group path and the legacy single-node path.
func (d *FabricXDeployer) pickOrdererRouter(ctx context.Context, cfg *types.FabricXNetworkConfig, preferOrgID int64) (string, []byte, error) {
	type pick struct {
		groupID int64
		nodeID  int64
	}
	var chosen pick
	for _, o := range cfg.Organizations {
		if o.OrdererNodeGroupID == 0 && o.OrdererNodeID == 0 {
			continue
		}
		candidate := pick{groupID: o.OrdererNodeGroupID, nodeID: o.OrdererNodeID}
		if preferOrgID != 0 && o.ID == preferOrgID {
			chosen = candidate
			break
		}
		if chosen.groupID == 0 && chosen.nodeID == 0 {
			chosen = candidate
		}
	}
	if chosen.groupID == 0 && chosen.nodeID == 0 {
		return "", nil, fmt.Errorf("no orderer group node found in network config")
	}

	var deployCfg nodetypes.FabricXOrdererGroupDeploymentConfig
	if chosen.groupID != 0 {
		grp, err := d.db.GetNodeGroup(ctx, chosen.groupID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get orderer node_group %d: %w", chosen.groupID, err)
		}
		if !grp.DeploymentConfig.Valid {
			return "", nil, fmt.Errorf("orderer node_group %d has no deployment config", chosen.groupID)
		}
		if err := json.Unmarshal([]byte(grp.DeploymentConfig.String), &deployCfg); err != nil {
			return "", nil, fmt.Errorf("failed to parse node_group deployment config: %w", err)
		}
	} else {
		dbNode, err := d.db.GetNode(ctx, chosen.nodeID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get orderer node %d: %w", chosen.nodeID, err)
		}
		if !dbNode.DeploymentConfig.Valid {
			return "", nil, fmt.Errorf("orderer node %d has no deployment config", chosen.nodeID)
		}
		if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &deployCfg); err != nil {
			return "", nil, fmt.Errorf("failed to parse orderer deployment config: %w", err)
		}
	}
	host := deployCfg.ExternalIP
	if host == "" {
		host = "localhost"
	}
	// Local-dev: the backend process runs on the host, and Docker Desktop on
	// macOS/Windows publishes ports on 127.0.0.1 only. Dialing the configured
	// LAN externalIP from the host typically hairpins and fails, so substitute
	// loopback. This mirrors the host.docker.internal swap we do for
	// container-originated connections in the genesis config.
	if resolveLocalDev(cfg, d.configService) {
		host = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", host, deployCfg.RouterPort)
	return addr, []byte(deployCfg.TLSCACert), nil
}

// pickCommitterSidecar returns the sidecar address + TLS CA for the submitter
// org (preferred) or the first committer in the config.
// The sidecar listens in plaintext gRPC (fabric-x-committer uses insecure
// connections everywhere internally), so we return an empty TLS CA.
//
// Supports both committer shapes: the new node_group path (resolves the group's
// shared FabricXCommitterDeploymentConfig to get host/SidecarPort) and the
// legacy monolithic committer node path.
func (d *FabricXDeployer) pickCommitterSidecar(ctx context.Context, cfg *types.FabricXNetworkConfig, preferOrgID int64) (string, []byte, error) {
	// (groupID, nodeID) candidate; only one is set per org.
	var chosenGroupID, chosenNodeID int64
	for _, o := range cfg.Organizations {
		if o.CommitterNodeGroupID == 0 && o.CommitterNodeID == 0 {
			continue
		}
		if preferOrgID != 0 && o.ID == preferOrgID {
			chosenGroupID = o.CommitterNodeGroupID
			chosenNodeID = o.CommitterNodeID
			break
		}
		if chosenGroupID == 0 && chosenNodeID == 0 {
			chosenGroupID = o.CommitterNodeGroupID
			chosenNodeID = o.CommitterNodeID
		}
	}
	if chosenGroupID == 0 && chosenNodeID == 0 {
		return "", nil, fmt.Errorf("no committer node found")
	}

	var deployCfg nodetypes.FabricXCommitterDeploymentConfig
	if chosenGroupID != 0 {
		grp, err := d.db.GetNodeGroup(ctx, chosenGroupID)
		if err != nil {
			return "", nil, fmt.Errorf("get committer node_group %d: %w", chosenGroupID, err)
		}
		if !grp.DeploymentConfig.Valid {
			return "", nil, fmt.Errorf("committer node_group %d has no deployment config", chosenGroupID)
		}
		if err := json.Unmarshal([]byte(grp.DeploymentConfig.String), &deployCfg); err != nil {
			return "", nil, err
		}
	} else {
		dbNode, err := d.db.GetNode(ctx, chosenNodeID)
		if err != nil {
			return "", nil, err
		}
		if !dbNode.DeploymentConfig.Valid {
			return "", nil, fmt.Errorf("committer %d has no deployment config", chosenNodeID)
		}
		if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &deployCfg); err != nil {
			return "", nil, err
		}
	}

	host := deployCfg.ExternalIP
	if host == "" {
		host = "localhost"
	}
	// See pickOrdererRouter: host-originated dials under Docker Desktop must
	// go through loopback rather than the configured LAN IP.
	if resolveLocalDev(cfg, d.configService) {
		host = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", host, deployCfg.SidecarPort)
	// Sidecar runs plaintext gRPC — no TLS CA.
	return addr, nil, nil
}

// --- pure helpers (testable without DB) -------------------------------------

func buildNamespaceTx(policyScheme string, policy []byte, nsID string, nsVersion int) *applicationpb.Tx {
	writeToMeta := &applicationpb.TxNamespace{
		NsId:       committerpb.MetaNamespaceID,
		NsVersion:  0,
		ReadWrites: make([]*applicationpb.ReadWrite, 0, 1),
	}

	nsPolicy := &applicationpb.NamespacePolicy{
		Rule: &applicationpb.NamespacePolicy_ThresholdRule{
			ThresholdRule: &applicationpb.ThresholdRule{
				Scheme:    policyScheme,
				PublicKey: policy,
			},
		},
	}
	policyBytes, _ := proto.Marshal(nsPolicy)

	rw := &applicationpb.ReadWrite{Key: []byte(nsID), Value: policyBytes}
	if nsVersion >= 0 {
		v := uint64(nsVersion)
		rw.Version = &v
	}
	writeToMeta.ReadWrites = append(writeToMeta.ReadWrites, rw)

	return &applicationpb.Tx{Namespaces: []*applicationpb.TxNamespace{writeToMeta}}
}

func signNamespaceEnvelope(signer adminidentity.SigningIdentity, channel string, tx *applicationpb.Tx) (*cb.Envelope, string, error) {
	sigHdr, err := protoutil.NewSignatureHeader(signer)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build signature header: %w", err)
	}
	txID := protoutil.ComputeTxID(sigHdr.Nonce, sigHdr.Creator)

	// Pull MSPID + cert bytes out of the creator for the Endorsement.Identity.
	var serialized msppb_fabric.SerializedIdentity
	if err := proto.Unmarshal(sigHdr.Creator, &serialized); err != nil {
		return nil, "", fmt.Errorf("invalid serialized identity: %w", err)
	}

	tx.Endorsements = make([]*applicationpb.Endorsements, len(tx.GetNamespaces()))
	for idx, ns := range tx.GetNamespaces() {
		msg, err := ns.ASN1Marshal(txID)
		if err != nil {
			return nil, "", fmt.Errorf("asn1 marshal: %w", err)
		}
		sig, err := signer.Sign(msg)
		if err != nil {
			return nil, "", fmt.Errorf("sign tx namespace: %w", err)
		}
		tx.Endorsements[idx] = &applicationpb.Endorsements{
			EndorsementsWithIdentity: []*applicationpb.EndorsementWithIdentity{
				{
					Endorsement: sig,
					Identity:    msppb.NewIdentity(serialized.Mspid, serialized.IdBytes),
				},
			},
		}
	}

	channelHdr := protoutil.MakeChannelHeader(cb.HeaderType_MESSAGE, 0, channel, 0)
	channelHdr.TxId = txID
	payloadHdr := protoutil.MakePayloadHeader(channelHdr, sigHdr)

	txBytes, err := proto.Marshal(tx)
	if err != nil {
		return nil, "", err
	}
	payloadBytes, err := proto.Marshal(&cb.Payload{Header: payloadHdr, Data: txBytes})
	if err != nil {
		return nil, "", err
	}
	envSig, err := signer.Sign(payloadBytes)
	if err != nil {
		return nil, "", fmt.Errorf("sign envelope: %w", err)
	}
	return &cb.Envelope{Payload: payloadBytes, Signature: envSig}, txID, nil
}

func broadcastEnvelope(ctx context.Context, addr string, tlsCA []byte, env *cb.Envelope) error {
	conn, err := dialGRPC(ctx, addr, tlsCA)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := ab.NewAtomicBroadcastClient(conn).Broadcast(ctx)
	if err != nil {
		return fmt.Errorf("open broadcast stream: %w", err)
	}
	if err := client.Send(env); err != nil {
		return fmt.Errorf("send envelope: %w", err)
	}
	resp, err := client.Recv()
	if err != nil {
		return fmt.Errorf("recv response: %w", err)
	}
	if resp.GetStatus() != cb.Status_SUCCESS {
		return fmt.Errorf("orderer rejected tx: status=%s info=%s", resp.GetStatus(), resp.GetInfo())
	}
	return nil
}

// txStatusWaiter subscribes to the committer-sidecar's Notifier stream for a
// single TxID. Subscribe before broadcasting so we don't miss the event.
type txStatusWaiter struct {
	conn    *grpc.ClientConn
	stream  committerpb.Notifier_OpenNotificationStreamClient
	txID    string
	timeout time.Duration
}

func openTxStatusWaiter(ctx context.Context, sidecarAddr, txID string, timeout time.Duration) (*txStatusWaiter, error) {
	// Sidecar notifier is plaintext gRPC.
	conn, err := dialGRPC(ctx, sidecarAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("dial sidecar: %w", err)
	}
	stream, err := committerpb.NewNotifierClient(conn).OpenNotificationStream(ctx)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open notifier stream: %w", err)
	}
	req := &committerpb.NotificationRequest{
		TxStatusRequest: &committerpb.TxIDsBatch{TxIds: []string{txID}},
		Timeout:         durationpb.New(timeout),
	}
	if err := stream.Send(req); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send subscription: %w", err)
	}
	return &txStatusWaiter{conn: conn, stream: stream, txID: txID, timeout: timeout}, nil
}

// WaitForCommit blocks until the committer reports the TxID's status or the
// timeout elapses. Any status other than COMMITTED is returned verbatim
// so the caller can see why the tx was rejected.
func (w *txStatusWaiter) WaitForCommit() error {
	deadline := time.Now().Add(w.timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for tx %s", w.timeout, w.txID)
		}
		resp, err := w.stream.Recv()
		if err != nil {
			return fmt.Errorf("notifier recv: %w", err)
		}
		// Timeouts are signaled by the server in a separate field.
		for _, tid := range resp.GetTimeoutTxIds() {
			if tid == w.txID {
				return fmt.Errorf("committer timed out waiting for tx %s", w.txID)
			}
		}
		for _, ev := range resp.GetTxStatusEvents() {
			if ev.GetRef().GetTxId() == w.txID {
				st := ev.GetStatus()
				if st != committerpb.Status_COMMITTED {
					return fmt.Errorf("tx %s rejected: status=%s", w.txID, st)
				}
				return nil
			}
		}
	}
}

func (w *txStatusWaiter) Close() {
	if w.stream != nil {
		_ = w.stream.CloseSend()
	}
	if w.conn != nil {
		_ = w.conn.Close()
	}
}

func dialGRPC(ctx context.Context, addr string, tlsCA []byte) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	opts := []grpc.DialOption{grpc.WithBlock()}
	if len(tlsCA) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(tlsCA) {
			return nil, fmt.Errorf("invalid TLS CA pem")
		}
		// When local-dev swaps externalIP to 127.0.0.1, the dial host won't
		// match the orderer cert SANs (which include only localhost and
		// host.docker.internal). Pin ServerName to localhost so TLS still
		// verifies against the SAN list.
		tlsCfg := &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
		if host, _, err := net.SplitHostPort(addr); err == nil && host == "127.0.0.1" {
			tlsCfg.ServerName = "localhost"
		}
		creds := credentials.NewTLS(tlsCfg)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.DialContext(dialCtx, addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return conn, nil
}

func marshalPublicKey(cert *x509.Certificate) ([]byte, error) {
	var der []byte
	var err error
	switch k := cert.PublicKey.(type) {
	case *ecdsa.PublicKey:
		der, err = x509.MarshalPKIXPublicKey(k)
	default:
		der, err = x509.MarshalPKIXPublicKey(cert.PublicKey)
	}
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// decodePEMPublicKeyToPKIXPEM parses a PEM-encoded PKIX public key and
// re-encodes it as PEM("PUBLIC KEY", MarshalPKIXPublicKey(pub)). The output
// is shape-compatible with what marshalPublicKey produces from a certificate,
// ensuring the stored namespace policy bytes are always in the same format
// regardless of whether the key came from a cert or was supplied directly.
func decodePEMPublicKeyToPKIXPEM(pemBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX public key: %w", err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("marshal PKIX public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// decodePEMPublicKey parses a PEM-encoded public key into its DER bytes.
// Kept for future use (e.g., honoring a user-supplied verification key).
func decodePEMPublicKey(pemBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return x509.MarshalPKIXPublicKey(pub)
}
