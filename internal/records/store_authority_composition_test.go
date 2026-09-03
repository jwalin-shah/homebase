package records_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TASK-021 A0b treats StoreAuthority as an in-process capability whose root of
// trust is the production composition root. Production code must therefore
// mint the authority bundle exactly once, in cmd/homebase, and must never use
// the deterministic test constructor outside tests.
func TestProductionAuthorityMintingIsCompositionRootOnly(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "../.."))
	fset := token.NewFileSet()
	var productionCalls []string
	var testConstructorCalls []string

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "NewStoreWithAuthorities":
				productionCalls = append(productionCalls, rel)
			case "NewStoreWithClockAndAuthorities":
				testConstructorCalls = append(testConstructorCalls, rel)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go sources: %v", err)
	}

	if len(testConstructorCalls) != 0 {
		t.Fatalf("deterministic authority constructor is test-only but production calls exist: %v", testConstructorCalls)
	}
	if len(productionCalls) != 1 || filepath.ToSlash(productionCalls[0]) != "cmd/homebase/main.go" {
		t.Fatalf("production authority minting must occur exactly once in cmd/homebase/main.go; calls=%v", productionCalls)
	}
}
