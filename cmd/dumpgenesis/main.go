//go:build dumpgenesis

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"google.golang.org/protobuf/proto"
)

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	block := &common.Block{}
	if err := proto.Unmarshal(data, block); err != nil {
		panic(err)
	}
	env, err := protoutil.ExtractEnvelope(block, 0)
	if err != nil {
		panic(err)
	}
	bundle, err := channelconfig.NewBundleFromEnvelope(env, factory.GetDefault())
	if err != nil {
		panic(err)
	}
	oc, ok := bundle.OrdererConfig()
	if !ok {
		panic("no orderer config")
	}
	fmt.Println("=== ConsenterMapping ===")
	cs := oc.Consenters()
	for i, c := range cs {
		entry := map[string]any{
			"idx":              i,
			"id":               c.Id,
			"host":             c.Host,
			"port":             c.Port,
			"msp_id":           c.MspId,
			"identity_len":     len(c.Identity),
			"client_tls_len":   len(c.ClientTlsCert),
			"server_tls_len":   len(c.ServerTlsCert),
			"client_tls_first": firstLine(c.ClientTlsCert),
			"server_tls_first": firstLine(c.ServerTlsCert),
		}
		b, _ := json.MarshalIndent(entry, "", "  ")
		fmt.Println(string(b))
	}
	fmt.Println("=== Orderer organizations ===")
	for mspID, org := range oc.Organizations() {
		m := org.MSP()
		var tlsRootLens []int
		if m != nil {
			for _, r := range m.GetTLSRootCerts() {
				tlsRootLens = append(tlsRootLens, len(r))
			}
		}
		endpoints := org.Endpoints()
		fmt.Printf("%s: tls_root_lens=%v endpoints=%v\n", mspID, tlsRootLens, endpoints)
	}
}

func firstLine(b []byte) string {
	for i, c := range b {
		if c == '\n' {
			return string(b[:i])
		}
	}
	if len(b) > 40 {
		return string(b[:40])
	}
	return string(b)
}
