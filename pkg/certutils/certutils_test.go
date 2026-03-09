package certutils

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestParsePrivateKeyPEM_PKCS8_EC(t *testing.T) {
	// Generate an ECDSA key and encode as PKCS#8
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal PKCS8: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	parsed, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM failed: %v", err)
	}

	if _, ok := parsed.(*ecdsa.PrivateKey); !ok {
		t.Errorf("expected *ecdsa.PrivateKey, got %T", parsed)
	}
}

func TestParsePrivateKeyPEM_SEC1_EC(t *testing.T) {
	// Generate an ECDSA key and encode as SEC1 (EC PRIVATE KEY)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}

	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal EC key: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})

	parsed, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM failed: %v", err)
	}

	if _, ok := parsed.(*ecdsa.PrivateKey); !ok {
		t.Errorf("expected *ecdsa.PrivateKey, got %T", parsed)
	}
}

func TestParsePrivateKeyPEM_PKCS1_RSA(t *testing.T) {
	// Generate an RSA key and encode as PKCS#1
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	der := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})

	parsed, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM failed: %v", err)
	}

	if _, ok := parsed.(*rsa.PrivateKey); !ok {
		t.Errorf("expected *rsa.PrivateKey, got %T", parsed)
	}
}

func TestParsePrivateKeyPEM_PKCS8_RSA(t *testing.T) {
	// Generate an RSA key and encode as PKCS#8
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal PKCS8: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	parsed, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM failed: %v", err)
	}

	if _, ok := parsed.(*rsa.PrivateKey); !ok {
		t.Errorf("expected *rsa.PrivateKey, got %T", parsed)
	}
}

func TestParsePrivateKeyPEM_PKCS8_Ed25519(t *testing.T) {
	// Generate an Ed25519 key and encode as PKCS#8
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal PKCS8: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	parsed, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM failed: %v", err)
	}

	if _, ok := parsed.(ed25519.PrivateKey); !ok {
		t.Errorf("expected ed25519.PrivateKey, got %T", parsed)
	}
}

func TestParsePrivateKeyPEM_FallbackParsing(t *testing.T) {
	// Encode an ECDSA key as PKCS#8 but with a non-standard PEM type header
	// to exercise the fallback parsing logic
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal PKCS8: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "UNKNOWN KEY TYPE", Bytes: der})

	parsed, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM fallback failed: %v", err)
	}

	if _, ok := parsed.(*ecdsa.PrivateKey); !ok {
		t.Errorf("expected *ecdsa.PrivateKey from fallback, got %T", parsed)
	}
}

func TestParsePrivateKeyPEM_InvalidPEM(t *testing.T) {
	_, err := ParsePrivateKeyPEM([]byte("not a pem"))
	if err == nil {
		t.Error("expected error for invalid PEM, got nil")
	}
}

func TestParsePrivateKeyPEM_EmptyInput(t *testing.T) {
	_, err := ParsePrivateKeyPEM([]byte{})
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestParsePrivateKeyPEM_UnsupportedType(t *testing.T) {
	// Create a PEM block with garbage data that can't be parsed by any parser
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "UNKNOWN KEY TYPE", Bytes: []byte("garbage data")})

	_, err := ParsePrivateKeyPEM(pemBytes)
	if err == nil {
		t.Error("expected error for unsupported key type, got nil")
	}
}
