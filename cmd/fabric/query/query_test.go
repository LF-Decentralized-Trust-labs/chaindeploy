package query

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// TestNoMathRandImport_QueryGo uses go/parser to statically verify that
// query.go does not import "math/rand", ensuring the crypto/rand security fix
// is preserved.
func TestNoMathRandImport_QueryGo(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path via runtime.Caller")
	}
	sourceFile := filepath.Join(filepath.Dir(currentFile), "query.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, sourceFile, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse query.go: %v", err)
	}

	for _, imp := range f.Imports {
		importPath := imp.Path.Value
		if importPath == `"math/rand"` || importPath == `"math/rand/v2"` {
			t.Errorf("query.go must not import %s — use crypto/rand for secure random peer selection", importPath)
		}
	}
}
