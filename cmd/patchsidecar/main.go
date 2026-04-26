package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"google.golang.org/protobuf/proto"
)

func extract(genesisBlock []byte) (map[string][][]byte, error) {
	block := &common.Block{}
	if err := proto.Unmarshal(genesisBlock, block); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	envelope, err := protoutil.ExtractEnvelope(block, 0)
	if err != nil {
		return nil, err
	}
	bundle, err := channelconfig.NewBundleFromEnvelope(envelope, factory.GetDefault())
	if err != nil {
		return nil, err
	}
	oc, ok := bundle.OrdererConfig()
	if !ok {
		return nil, fmt.Errorf("no orderer config")
	}
	out := map[string][][]byte{}
	for mspID, org := range oc.Organizations() {
		m := org.MSP()
		if m == nil {
			continue
		}
		roots := m.GetTLSRootCerts()
		if len(roots) > 0 {
			out[mspID] = roots
		}
	}
	return out, nil
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: patch <genesis> <tlscacertsDir> <configOutYaml>")
		os.Exit(1)
	}
	genesisPath := os.Args[1]
	tlsDir := os.Args[2]
	cfgOut := os.Args[3]

	data, err := os.ReadFile(genesisPath)
	if err != nil {
		panic(err)
	}
	roots, err := extract(data)
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		panic(err)
	}
	// drop any stale party- files
	entries, _ := os.ReadDir(tlsDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "party-") {
			_ = os.Remove(filepath.Join(tlsDir, e.Name()))
		}
	}
	var paths []string
	for mspID, certs := range roots {
		for i, c := range certs {
			fname := fmt.Sprintf("party-%s-%d.pem", strings.ToLower(mspID), i)
			if err := os.WriteFile(filepath.Join(tlsDir, fname), c, 0644); err != nil {
				panic(err)
			}
			paths = append(paths, filepath.Join("/var/hyperledger/fabricx/msp/tlscacerts", fname))
		}
	}
	// Rewrite the sidecar YAML to inject root-ca-paths under orderer:
	y, err := os.ReadFile(cfgOut)
	if err != nil {
		panic(err)
	}
	text := string(y)
	// Remove any prior connection block we injected.
	if idx := strings.Index(text, "\n  connection:\n    root-ca-paths:\n"); idx >= 0 {
		// crude: find end (blank line or next top-level key). The committer: line starts at col 0.
		tail := text[idx+1:]
		end := strings.Index(tail, "\ncommitter:\n")
		if end >= 0 {
			text = text[:idx] + text[idx+1+end:]
		}
	}
	// Inject after the identity block (before committer:)
	insert := "  connection:\n    root-ca-paths:\n"
	for _, p := range paths {
		insert += fmt.Sprintf("      - %s\n", p)
	}
	text = strings.Replace(text, "committer:\n", insert+"committer:\n", 1)
	if err := os.WriteFile(cfgOut, []byte(text), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %d CA files to %s; injected %d root-ca-paths into %s\n",
		len(paths), tlsDir, len(paths), cfgOut)
}
