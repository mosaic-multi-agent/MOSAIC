package main

// vocabulary_isolation_test.go holds two guards over Deploy's own production
// code, which used to live next to the agent-format layer and stayed behind when
// the layer moved to mosaic-common:
//   - no literal "mosaic_" string: field names come from mosaic-common/agentfields;
//   - the test-only agent format is invisible: no import of the testjson package,
//     no use of its format id literal, no call of RegisterTestOnly.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deployProductionFiles returns every non-test Go file of the Deploy module
// (vendor and testdata excluded).
func deployProductionFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join("..", ".."), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "testdata" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk Deploy module: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("found no production Go files in the Deploy module")
	}
	return files
}

// TestDeployProduction_NoMosaicLiteral verifies that no production file of the
// Deploy module hard-codes a "mosaic_"-prefixed field name; every site derives
// the name from the agentfields registry.
func TestDeployProduction_NoMosaicLiteral(t *testing.T) {
	for _, path := range deployProductionFiles(t) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if bytes.Contains(content, []byte(`"mosaic_`)) {
			t.Errorf("production file %s contains a literal \"mosaic_\" string; derive field names from mosaic-common/agentfields", path)
		}
	}
}

// TestDeployProduction_TestFormatNotReferenced verifies that Deploy production
// code never reaches the test-only agent format.
func TestDeployProduction_TestFormatNotReferenced(t *testing.T) {
	const testjsonImport = "mosaic-common/agentformat/testjson"
	const testFormatLiteral = "mosaic-test-json"
	const registerTestOnly = "RegisterTestOnly"

	for _, path := range deployProductionFiles(t) {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			if strings.Trim(imp.Path.Value, `"`) == testjsonImport {
				t.Errorf("%s: imports the test-only agent format package", path)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING && strings.Contains(x.Value, testFormatLiteral) {
					t.Errorf("%s: contains the test-only format id literal", path)
				}
			case *ast.SelectorExpr:
				if x.Sel.Name == registerTestOnly {
					t.Errorf("%s: references %s", path, registerTestOnly)
				}
			}
			return true
		})
	}
}
