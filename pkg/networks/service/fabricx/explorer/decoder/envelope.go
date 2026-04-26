// Package decoder unmarshals FabricX envelopes and blocks into a
// UI-friendly JSON shape. Ported from fabric-x-operator/explorer/internal/decoder.
package decoder

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-common/api/applicationpb"
	"github.com/hyperledger/fabric-x-common/api/msppb"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"google.golang.org/protobuf/proto"
)

type DecodedTx struct {
	TxID       string            `json:"txId"`
	ChannelID  string            `json:"channelId"`
	Timestamp  time.Time         `json:"timestamp"`
	Type       string            `json:"type"`
	Namespaces []DecodedNS       `json:"namespaces"`
	Endorsers  []DecodedEndorser `json:"endorsers"`
}

type DecodedNS struct {
	NsID        string         `json:"nsId"`
	NsVersion   uint64         `json:"nsVersion"`
	Reads       []DecodedRead  `json:"reads,omitempty"`
	ReadWrites  []DecodedRW    `json:"readWrites,omitempty"`
	BlindWrites []DecodedWrite `json:"blindWrites,omitempty"`
}

type DecodedRead struct {
	Key      string  `json:"key"`
	Version  *uint64 `json:"version"`
	KeyLabel string  `json:"keyLabel,omitempty"`
}

type DecodedRW struct {
	Key       string  `json:"key"`
	Version   *uint64 `json:"version"`
	Value     string  `json:"value"`
	KeyLabel  string  `json:"keyLabel,omitempty"`
	ValueInfo string  `json:"valueInfo,omitempty"`
}

type DecodedWrite struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	KeyLabel  string `json:"keyLabel,omitempty"`
	ValueInfo string `json:"valueInfo,omitempty"`
}

type DecodedEndorser struct {
	MspID   string `json:"mspId"`
	Subject string `json:"subject,omitempty"`
}

func DecodeEnvelope(env *cb.Envelope) (*DecodedTx, error) {
	payload, err := protoutil.UnmarshalPayload(env.GetPayload())
	if err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	chdr, err := protoutil.UnmarshalChannelHeader(payload.GetHeader().GetChannelHeader())
	if err != nil {
		return nil, fmt.Errorf("unmarshal channel header: %w", err)
	}

	tx := &DecodedTx{
		TxID:      chdr.GetTxId(),
		ChannelID: chdr.GetChannelId(),
		Timestamp: chdr.GetTimestamp().AsTime(),
	}

	var appTx applicationpb.Tx
	if err := proto.Unmarshal(payload.GetData(), &appTx); err != nil {
		tx.Type = "config"
		return tx, nil
	}
	tx.Type = "application"

	for _, ns := range appTx.GetNamespaces() {
		dns := DecodedNS{
			NsID:      ns.GetNsId(),
			NsVersion: ns.GetNsVersion(),
		}
		for _, r := range ns.GetReadsOnly() {
			dns.Reads = append(dns.Reads, DecodedRead{
				Key:      formatKey(r.GetKey()),
				Version:  r.Version,
				KeyLabel: labelKey(r.GetKey()),
			})
		}
		for _, rw := range ns.GetReadWrites() {
			dns.ReadWrites = append(dns.ReadWrites, DecodedRW{
				Key:       formatKey(rw.GetKey()),
				Version:   rw.Version,
				Value:     truncateValue(rw.GetValue()),
				KeyLabel:  labelKey(rw.GetKey()),
				ValueInfo: describeValue(rw.GetKey(), rw.GetValue()),
			})
		}
		for _, w := range ns.GetBlindWrites() {
			dns.BlindWrites = append(dns.BlindWrites, DecodedWrite{
				Key:       formatKey(w.GetKey()),
				Value:     truncateValue(w.GetValue()),
				KeyLabel:  labelKey(w.GetKey()),
				ValueInfo: describeValue(w.GetKey(), w.GetValue()),
			})
		}
		tx.Namespaces = append(tx.Namespaces, dns)
	}

	for _, endorsements := range appTx.GetEndorsements() {
		for _, e := range endorsements.GetEndorsementsWithIdentity() {
			de := decodeEndorser(e.GetIdentity())
			tx.Endorsers = append(tx.Endorsers, de)
		}
	}

	return tx, nil
}

func decodeEndorser(id *msppb.Identity) DecodedEndorser {
	if id == nil {
		return DecodedEndorser{MspID: "unknown"}
	}
	de := DecodedEndorser{MspID: id.GetMspId()}

	block, _ := pem.Decode(id.GetCertificate())
	if block != nil {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			de.Subject = cert.Subject.CommonName
		}
	}
	return de
}

// labelKey interprets well-known key patterns from the token namespace.
func labelKey(key []byte) string {
	parts := splitNullKey(key)
	if len(parts) == 0 {
		return ""
	}
	switch {
	case len(parts) >= 1 && parts[0] == "se":
		return "Public Parameters"
	case len(parts) >= 1 && parts[0] == "seh":
		return "Public Parameters Hash"
	case len(parts) >= 2 && parts[0] == "tr":
		return "Token Request (txid: " + shortID(parts[1]) + ")"
	case len(parts) >= 2 && parts[0] == "osn":
		return "Ownership Record (owner: " + shortID(parts[1]) + ")"
	case len(parts) >= 2 && len(parts[0]) == 64:
		return "Token Output (txid: " + shortID(parts[0]) + ", index: " + parts[1] + ")"
	default:
		return ""
	}
}

// describeValue provides a human-readable summary of the value.
func describeValue(key []byte, value []byte) string {
	parts := splitNullKey(key)
	if len(parts) == 0 {
		return ""
	}
	switch {
	case len(parts) >= 2 && parts[0] == "tr":
		return fmt.Sprintf("Token request hash (%d bytes)", len(value))
	case len(parts) >= 2 && parts[0] == "osn":
		if len(value) == 1 && value[0] == 0x01 {
			return "Active ownership flag"
		}
		return fmt.Sprintf("Ownership state (%d bytes)", len(value))
	case len(parts) >= 2 && len(parts[0]) == 64:
		return fmt.Sprintf("Serialized token output (%d bytes) — contains Pedersen commitment (hidden amount) + owner public key", len(value))
	case len(parts) >= 1 && parts[0] == "se":
		return fmt.Sprintf("Public parameters (%d bytes)", len(value))
	case len(parts) >= 1 && parts[0] == "seh":
		return fmt.Sprintf("PP hash (%d bytes)", len(value))
	default:
		return fmt.Sprintf("%d bytes", len(value))
	}
}

func splitNullKey(key []byte) []string {
	var parts []string
	current := []byte{}
	for _, b := range key {
		if b == 0x00 {
			if len(current) > 0 {
				parts = append(parts, string(current))
			}
			current = nil
		} else {
			current = append(current, b)
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12] + "..."
	}
	return id
}

func formatKey(key []byte) string {
	if len(key) == 0 {
		return ""
	}
	for _, b := range key {
		if b < 0x20 && b != 0x00 {
			return hex.EncodeToString(key)
		}
	}
	out := make([]byte, len(key))
	for i, b := range key {
		if b == 0x00 {
			out[i] = '|'
		} else {
			out[i] = b
		}
	}
	return string(out)
}

func truncateValue(val []byte) string {
	if len(val) <= 256 {
		return hex.EncodeToString(val)
	}
	return hex.EncodeToString(val[:256]) + fmt.Sprintf("...(%d bytes)", len(val))
}
