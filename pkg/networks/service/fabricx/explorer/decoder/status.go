package decoder

import "github.com/hyperledger/fabric-x-common/api/committerpb"

var statusLabels = map[committerpb.Status]string{
	committerpb.Status_STATUS_UNSPECIFIED:                        "UNSPECIFIED",
	committerpb.Status_COMMITTED:                                 "COMMITTED",
	committerpb.Status_ABORTED_SIGNATURE_INVALID:                 "ABORTED_SIGNATURE_INVALID",
	committerpb.Status_ABORTED_MVCC_CONFLICT:                     "ABORTED_MVCC_CONFLICT",
	committerpb.Status_REJECTED_DUPLICATE_TX_ID:                  "REJECTED_DUPLICATE_TX_ID",
	committerpb.Status_MALFORMED_BAD_ENVELOPE:                    "MALFORMED_BAD_ENVELOPE",
	committerpb.Status_MALFORMED_MISSING_TX_ID:                   "MALFORMED_MISSING_TX_ID",
	committerpb.Status_MALFORMED_UNSUPPORTED_ENVELOPE_PAYLOAD:    "MALFORMED_UNSUPPORTED_ENVELOPE_PAYLOAD",
	committerpb.Status_MALFORMED_BAD_ENVELOPE_PAYLOAD:            "MALFORMED_BAD_ENVELOPE_PAYLOAD",
	committerpb.Status_MALFORMED_TX_ID_CONFLICT:                  "MALFORMED_TX_ID_CONFLICT",
	committerpb.Status_MALFORMED_EMPTY_NAMESPACES:                "MALFORMED_EMPTY_NAMESPACES",
	committerpb.Status_MALFORMED_DUPLICATE_NAMESPACE:             "MALFORMED_DUPLICATE_NAMESPACE",
	committerpb.Status_MALFORMED_NAMESPACE_ID_INVALID:            "MALFORMED_NAMESPACE_ID_INVALID",
	committerpb.Status_MALFORMED_BLIND_WRITES_NOT_ALLOWED:        "MALFORMED_BLIND_WRITES_NOT_ALLOWED",
	committerpb.Status_MALFORMED_NO_WRITES:                       "MALFORMED_NO_WRITES",
	committerpb.Status_MALFORMED_EMPTY_KEY:                       "MALFORMED_EMPTY_KEY",
	committerpb.Status_MALFORMED_DUPLICATE_KEY_IN_READ_WRITE_SET: "MALFORMED_DUPLICATE_KEY_IN_READ_WRITE_SET",
	committerpb.Status_MALFORMED_MISSING_SIGNATURE:               "MALFORMED_MISSING_SIGNATURE",
	committerpb.Status_MALFORMED_NAMESPACE_POLICY_INVALID:        "MALFORMED_NAMESPACE_POLICY_INVALID",
	committerpb.Status_MALFORMED_CONFIG_TX_INVALID:               "MALFORMED_CONFIG_TX_INVALID",
}

func StatusLabel(s committerpb.Status) string {
	if label, ok := statusLabels[s]; ok {
		return label
	}
	return s.String()
}

func IsCommitted(s committerpb.Status) bool {
	return s == committerpb.Status_COMMITTED
}
