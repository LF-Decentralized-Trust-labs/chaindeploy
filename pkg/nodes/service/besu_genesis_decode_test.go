package service

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestDecodeBesuGenesisFile_Base64 covers the canonical case: networks
// persist genesis as base64-encoded JSON (see
// pkg/networks/service/service.go), and the helper must hand the besu
// node service the decoded JSON before it reaches validateGenesisFile.
func TestDecodeBesuGenesisFile_Base64(t *testing.T) {
	rawJSON := `{"config":{"chainId":1337},"alloc":{},"difficulty":"0x1","gasLimit":"0x1000000"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(rawJSON))

	got, err := decodeBesuGenesisFile(encoded)
	if err != nil {
		t.Fatalf("decodeBesuGenesisFile returned error: %v", err)
	}
	if got != rawJSON {
		t.Errorf("decoded genesis mismatch:\n  got:  %q\n  want: %q", got, rawJSON)
	}
}

// TestDecodeBesuGenesisFile_RawJSONPassthrough guards against a regression
// for any in-memory caller that hands us already-decoded JSON. We don't
// want to mangle that input — base64 decoding fails, but the trimmed
// payload starts with '{' so we return it untouched.
func TestDecodeBesuGenesisFile_RawJSONPassthrough(t *testing.T) {
	rawJSON := `{"config":{"chainId":1337},"alloc":{},"difficulty":"0x1","gasLimit":"0x1000000"}`

	got, err := decodeBesuGenesisFile(rawJSON)
	if err != nil {
		t.Fatalf("decodeBesuGenesisFile returned error on raw JSON: %v", err)
	}
	if got != rawJSON {
		t.Errorf("raw JSON should be returned untouched:\n  got:  %q\n  want: %q", got, rawJSON)
	}
}

// TestDecodeBesuGenesisFile_Empty handles the empty-string case the same
// way the call sites already did before this change: empty in, empty out,
// no error. The downstream validator catches missing genesis content
// with its own dedicated error message.
func TestDecodeBesuGenesisFile_Empty(t *testing.T) {
	got, err := decodeBesuGenesisFile("")
	if err != nil {
		t.Fatalf("decodeBesuGenesisFile(\"\") returned error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

// TestDecodeBesuGenesisFile_Garbage rejects input that is neither valid
// base64 nor a JSON-shaped string. The error message names the field so
// operators can debug a corrupted network row.
func TestDecodeBesuGenesisFile_Garbage(t *testing.T) {
	_, err := decodeBesuGenesisFile("!!!not-base64-and-not-json!!!")
	if err == nil {
		t.Fatal("expected error for garbage input, got nil")
	}
	if !strings.Contains(err.Error(), "decode genesis block") {
		t.Errorf("error message should mention 'decode genesis block', got: %v", err)
	}
}
