package invoke

import (
	cryptorand "crypto/rand"
	"go/parser"
	"go/token"
	"math/big"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// TestCryptoRandPeerSelection_SinglePeer verifies that when there is exactly
// one peer, the crypto/rand selection always produces index 0.
func TestCryptoRandPeerSelection_SinglePeer(t *testing.T) {
	peers := []string{"peer0.org1.example.com"}

	for i := 0; i < 100; i++ {
		randIdx, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(peers))))
		if err != nil {
			t.Fatalf("crypto/rand.Int returned error: %v", err)
		}
		idx := randIdx.Int64()
		if idx != 0 {
			t.Fatalf("expected index 0 for single peer, got %d", idx)
		}
	}
}

// TestCryptoRandPeerSelection_TwoPeers verifies that when there are two peers,
// the crypto/rand selection always produces index 0 or 1.
func TestCryptoRandPeerSelection_TwoPeers(t *testing.T) {
	peers := []string{"peer0.org1.example.com", "peer1.org1.example.com"}
	seen := map[int64]bool{}

	for i := 0; i < 200; i++ {
		randIdx, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(peers))))
		if err != nil {
			t.Fatalf("crypto/rand.Int returned error: %v", err)
		}
		idx := randIdx.Int64()
		if idx < 0 || idx >= int64(len(peers)) {
			t.Fatalf("index %d out of bounds [0, %d)", idx, len(peers))
		}
		seen[idx] = true
	}

	// With 200 iterations and 2 choices, probability of never seeing one is ~2^-200.
	if len(seen) != 2 {
		t.Errorf("expected both indices to be selected at least once in 200 iterations, saw %v", seen)
	}
}

// TestCryptoRandPeerSelection_MultiplePeers verifies that crypto/rand produces
// indices within bounds for a larger peer list.
func TestCryptoRandPeerSelection_MultiplePeers(t *testing.T) {
	sizes := []int{3, 5, 10, 50}
	for _, size := range sizes {
		t.Run("size_"+strconv.Itoa(size), func(t *testing.T) {
			for i := 0; i < 500; i++ {
				randIdx, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(size)))
				if err != nil {
					t.Fatalf("crypto/rand.Int returned error: %v", err)
				}
				idx := randIdx.Int64()
				if idx < 0 || idx >= int64(size) {
					t.Fatalf("index %d out of bounds [0, %d)", idx, size)
				}
			}
		})
	}
}

// TestNoMathRandImport_InvokeGo uses go/parser to statically verify that
// invoke.go does not import "math/rand", ensuring the crypto/rand security fix
// is preserved.
func TestNoMathRandImport_InvokeGo(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path via runtime.Caller")
	}
	sourceFile := filepath.Join(filepath.Dir(currentFile), "invoke.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, sourceFile, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse invoke.go: %v", err)
	}

	for _, imp := range f.Imports {
		importPath := imp.Path.Value // includes surrounding quotes
		if importPath == `"math/rand"` || importPath == `"math/rand/v2"` {
			t.Errorf("invoke.go must not import %s — use crypto/rand for secure random peer selection", importPath)
		}
	}
}
