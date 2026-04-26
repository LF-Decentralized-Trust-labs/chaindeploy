package fabricx

import (
	"fmt"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"google.golang.org/protobuf/proto"
)

// extractOrdererTLSRootCerts parses a genesis block and returns the PEM-encoded
// TLS root CA certificates for every orderer organization, keyed by MSP ID.
// The sidecar uses these to verify TLS handshakes against each party's
// router/assembler when fetching blocks.
func extractOrdererTLSRootCerts(genesisBlock []byte) (map[string][][]byte, error) {
	block := &common.Block{}
	if err := proto.Unmarshal(genesisBlock, block); err != nil {
		return nil, fmt.Errorf("unmarshal genesis block: %w", err)
	}
	envelope, err := protoutil.ExtractEnvelope(block, 0)
	if err != nil {
		return nil, fmt.Errorf("extract envelope: %w", err)
	}
	bundle, err := channelconfig.NewBundleFromEnvelope(envelope, factory.GetDefault())
	if err != nil {
		return nil, fmt.Errorf("build config bundle: %w", err)
	}
	oc, ok := bundle.OrdererConfig()
	if !ok {
		return nil, fmt.Errorf("orderer config not found in genesis")
	}
	result := make(map[string][][]byte)
	for mspID, org := range oc.Organizations() {
		m := org.MSP()
		if m == nil {
			continue
		}
		roots := m.GetTLSRootCerts()
		if len(roots) == 0 {
			continue
		}
		result[mspID] = roots
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no TLS root certs found for any orderer org")
	}
	return result, nil
}
