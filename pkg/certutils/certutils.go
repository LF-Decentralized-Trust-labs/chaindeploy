package certutils

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

func EncodeX509Certificate(crt *x509.Certificate) []byte {
	pemPk := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crt.Raw,
	})
	return pemPk
}

func ParseX509Certificate(contents []byte) (*x509.Certificate, error) {
	if len(contents) == 0 {
		return nil, errors.New("certificate pem is empty")
	}
	block, _ := pem.Decode(contents)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return crt, nil
}

// ParsePrivateKeyPEM parses a PEM-encoded private key supporting PKCS#8, SEC1 EC, and PKCS#1 RSA formats.
// This is more flexible than gwidentity.PrivateKeyFromPEM which only handles ECDSA keys.
func ParsePrivateKeyPEM(privateKeyPEM []byte) (crypto.PrivateKey, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, errors.New("failed to parse private key PEM")
	}

	switch block.Type {
	case "PRIVATE KEY":
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		// Try all formats as fallback
		if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
			return k, nil
		}
		if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
			return k, nil
		}
		if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
			return k, nil
		}
		return nil, fmt.Errorf("unsupported private key type %q", block.Type)
	}
}
