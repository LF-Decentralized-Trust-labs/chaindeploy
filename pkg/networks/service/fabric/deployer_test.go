package fabric

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNoMathRandImport_FabricPackage uses go/parser to statically verify that
// no Go source file in the fabric deployer package imports "math/rand".
// This guards against regressions where a predictable PRNG could be used
// for security-sensitive peer selection.
func TestNoMathRandImport_FabricPackage(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path via runtime.Caller")
	}
	pkgDir := filepath.Dir(currentFile)

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("failed to read package directory: %v", err)
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		fullPath := filepath.Join(pkgDir, name)
		f, err := parser.ParseFile(fset, fullPath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", name, err)
		}

		for _, imp := range f.Imports {
			importPath := imp.Path.Value
			if importPath == `"math/rand"` || importPath == `"math/rand/v2"` {
				t.Errorf("%s must not import %s — use crypto/rand for secure random operations", name, importPath)
			}
		}
	}
}

// TestNoMathRandImport_InvokeAndQueryPackages scans the invoke and query
// command packages to ensure no math/rand imports exist, verifying the
// crypto/rand security fix for random peer selection.
func TestNoMathRandImport_InvokeAndQueryPackages(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path via runtime.Caller")
	}

	// Navigate from pkg/networks/service/fabric/ to cmd/fabric/
	repoRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..")
	cmdPackages := []string{
		filepath.Join(repoRoot, "cmd", "fabric", "invoke"),
		filepath.Join(repoRoot, "cmd", "fabric", "query"),
	}

	fset := token.NewFileSet()
	for _, pkgDir := range cmdPackages {
		entries, err := os.ReadDir(pkgDir)
		if err != nil {
			t.Fatalf("failed to read directory %s: %v", pkgDir, err)
		}

		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}

			fullPath := filepath.Join(pkgDir, name)
			f, err := parser.ParseFile(fset, fullPath, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", fullPath, err)
			}

			for _, imp := range f.Imports {
				importPath := imp.Path.Value
				if importPath == `"math/rand"` || importPath == `"math/rand/v2"` {
					t.Errorf("%s must not import %s — use crypto/rand for secure random peer selection", fullPath, importPath)
				}
			}
		}
	}
}
