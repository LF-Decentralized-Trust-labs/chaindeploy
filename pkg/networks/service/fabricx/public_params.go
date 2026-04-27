package fabricx

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"

	adminidentity "github.com/hyperledger/fabric-admin-sdk/pkg/identity"
	gwidentity "github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/hyperledger/fabric-x-common/api/applicationpb"
)

// Token-SDK chaincode-shim keys for public parameters storage.
// These are stable composite keys defined in
// fabric-token-sdk/token/services/network/common/rws/keys/keys.go:
//
//	compositeKeyNamespace = "\x00"
//	TokenSetupKeyPrefix   = "se"   → "\x00se\x00"
//	TokenSetupHashKeyPrefix = "seh" → "\x00seh\x00"
//
// Inlined here to avoid adding fabric-token-sdk to go.mod.
const (
	tokenSetupKey     = "\x00se\x00"
	tokenSetupHashKey = "\x00seh\x00"
)

// PublishPublicParamsOptions describes a request to publish ZK public
// parameters into a FabricX namespace.
type PublishPublicParamsOptions struct {
	// Name is the namespace name (required).
	Name string
	// SubmitterOrgID is the fabric_organizations.id whose admin key signs the
	// transaction (required).
	SubmitterOrgID int64
	// PublicParameters is the raw zkatdlog PP bytes (required).
	PublicParameters []byte
	// WaitForFinality, when true, opens a Deliver stream on the committer sidecar
	// and blocks until the tx is committed (or the timeout elapses).
	WaitForFinality bool
	// FinalityTimeout defaults to 2 minutes when zero.
	FinalityTimeout time.Duration
}

// PublishPublicParamsResult is returned to callers of PublishPublicParams.
type PublishPublicParamsResult struct {
	// TxID is the fabric transaction identifier that was broadcast.
	TxID string
	// Status is "broadcast" or "committed".
	Status string
}

// PublishPublicParams broadcasts the token-sdk public parameters into the
// named namespace on a FabricX channel, signed by the submitter org's admin
// key. This is an additive operation — it does not modify namespace policies.
func (d *FabricXDeployer) PublishPublicParams(ctx context.Context, networkID int64, opts PublishPublicParamsOptions) (*PublishPublicParamsResult, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("namespace name is required")
	}
	if opts.SubmitterOrgID == 0 {
		return nil, fmt.Errorf("submitter_org_id is required")
	}
	if len(opts.PublicParameters) == 0 {
		return nil, fmt.Errorf("public_parameters must not be empty")
	}
	if opts.FinalityTimeout == 0 {
		opts.FinalityTimeout = 2 * time.Minute
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

	// Resolve submitter org's admin signing identity.
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

	// Pick an orderer router.
	ordererAddr, ordererTLSCA, err := d.pickOrdererRouter(ctx, &cfg, opts.SubmitterOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to pick orderer: %w", err)
	}

	// Build and sign the PP transaction.
	tx := buildPublicParamsTx(opts.Name, opts.PublicParameters)
	env, txID, err := signNamespaceEnvelope(signer, cfg.ChannelName, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign public-params tx: %w", err)
	}

	// Pre-register finality waiter before broadcast so we don't miss the event.
	var waiter *txStatusWaiter
	if opts.WaitForFinality {
		committerAddr, _, ferr := d.pickCommitterSidecar(ctx, &cfg, opts.SubmitterOrgID)
		if ferr != nil {
			d.logger.Warn("Skipping finality wait: no committer sidecar available", "err", ferr)
		} else {
			w, werr := openTxStatusWaiter(ctx, committerAddr, txID, opts.FinalityTimeout)
			if werr != nil {
				return nil, fmt.Errorf("finality setup failed: %w", werr)
			}
			waiter = w
			defer waiter.Close()
		}
	}

	// Broadcast to orderer.
	if err := broadcastEnvelope(ctx, ordererAddr, ordererTLSCA, env); err != nil {
		return nil, fmt.Errorf("broadcast failed: %w", err)
	}
	d.logger.Info("Public-params tx broadcast accepted", "network", networkID, "namespace", opts.Name, "txID", txID)

	status := "broadcast"
	if waiter != nil {
		if err := waiter.WaitForCommit(); err != nil {
			return nil, fmt.Errorf("finality wait failed: %w", err)
		}
		status = "committed"
		d.logger.Info("Public-params tx committed", "network", networkID, "namespace", opts.Name, "txID", txID)
	}

	return &PublishPublicParamsResult{TxID: txID, Status: status}, nil
}

// buildPublicParamsTx constructs the applicationpb.Tx that writes the token-sdk
// public parameters into the named namespace. The shape mirrors what the
// token-sdk deployer does internally (createPublicParametersTx):
//
//	BlindWrites: setup key ("\x00se\x00")  → ppRaw
//	             setup hash key ("\x00seh\x00") → sha256(ppRaw)
//	ReadsOnly:   "initialized" (guards against re-init races)
func buildPublicParamsTx(nsName string, ppRaw []byte) *applicationpb.Tx {
	hash := sha256.Sum256(ppRaw)

	ns := &applicationpb.TxNamespace{
		NsId:      nsName,
		NsVersion: 0,
		ReadsOnly: []*applicationpb.Read{
			{Key: []byte("initialized")},
		},
		BlindWrites: []*applicationpb.Write{
			{Key: []byte(tokenSetupKey), Value: ppRaw},
			{Key: []byte(tokenSetupHashKey), Value: hash[:]},
		},
	}
	return &applicationpb.Tx{Namespaces: []*applicationpb.TxNamespace{ns}}
}
