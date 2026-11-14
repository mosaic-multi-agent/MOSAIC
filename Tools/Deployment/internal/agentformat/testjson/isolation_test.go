package testjson_test

// isolation_test.go guards that the test format is invisible to production code: no
// non-test Go file outside internal/agentformat imports the testjson package, contains
// its format id as a string literal, or references agentformat.RegisterTestOnly, and
// the production wiring package does not import testjson. The scanner is exercised on
// synthetic trees first, so a guard that never fires cannot pass unnoticed.

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

const (
	testjsonImportPath = "mosaic-deploy/internal/agentformat/testjson"
	testFormatLiteral  = "mosaic-test-json"
	registerTestOnly   = "RegisterTestOnly"
	agentformatDir     = "internal/agentformat"
	wiringDir          = "internal/agentformat/all"
)

// isolationViolations returns one message per violation found under moduleRoot.
func isolationViolations(t *testing.T, moduleRoot string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		out = append(out, fileViolations(t, path, rel)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", moduleRoot, err)
	}
	return out
}

func fileViolations(t *testing.T, path, rel string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	inAgentformat := strings.HasPrefix(rel, agentformatDir+"/")
	inWiring := strings.HasPrefix(rel, wiringDir+"/")
	var out []string
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) == testjsonImportPath && (!inAgentformat || inWiring) {
			out = append(out, rel+": imports the test format package")
		}
	}
	if inAgentformat {
		return out
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING && strings.Contains(x.Value, testFormatLiteral) {
				out = append(out, rel+": contains the test format id literal")
			}
		case *ast.SelectorExpr:
			if x.Sel.Name == registerTestOnly {
				out = append(out, rel+": references "+registerTestOnly)
			}
		}
		return true
	})
	return out
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestIsolationScanner_DetectsEachKindOfReference(t *testing.T) {
	cases := map[string]map[string]string{
		"import outside agentformat": {
			"internal/app/x.go": "package app\nimport _ \"" + testjsonImportPath + "\"\n",
		},
		"id literal outside agentformat": {
			"internal/harness/x.go": "package harness\nconst id = \"" + testFormatLiteral + "\"\n",
		},
		"RegisterTestOnly outside agentformat": {
			"cmd/x/main.go": "package main\nimport \"mosaic-deploy/internal/agentformat\"\nfunc main() { agentformat.RegisterTestOnly(\"a\", nil) }\n",
		},
		"wiring package imports the test format": {
			"internal/agentformat/all/all.go": "package all\nimport _ \"" + testjsonImportPath + "\"\n",
		},
	}
	for name, files := range cases {
		files := files
		t.Run(name, func(t *testing.T) {
			if v := isolationViolations(t, writeTree(t, files)); len(v) == 0 {
				t.Errorf("scanner found no violation")
			}
		})
	}
}

func TestIsolationScanner_AcceptsPermittedPlaces(t *testing.T) {
	root := writeTree(t, map[string]string{
		"internal/app/clean.go":                "package app\nconst x = 1\n",
		"internal/app/uses_test.go":            "package app\nimport _ \"" + testjsonImportPath + "\"\nconst id = \"" + testFormatLiteral + "\"\n",
		"internal/agentformat/registry.go":     "package agentformat\nfunc RegisterTestOnly() {}\nfunc f() { RegisterTestOnly() }\n",
		"internal/agentformat/testjson/tj.go":  "package testjson\nconst ID = \"" + testFormatLiteral + "\"\n",
		"internal/app/testdata/fixture/f.go":   "package fixture\nconst id = \"" + testFormatLiteral + "\"\n",
		"internal/agentformat/other/x/user.go": "package user\nimport _ \"" + testjsonImportPath + "\"\n",
	})

	if v := isolationViolations(t, root); len(v) != 0 {
		t.Errorf("scanner flagged permitted places: %v", v)
	}
}

func TestTestFormat_NotReferencedByProductionCodeOutsideAgentformat(t *testing.T) {
	moduleRoot := filepath.Join("..", "..", "..")

	if v := isolationViolations(t, moduleRoot); len(v) != 0 {
		t.Errorf("test format leaked into production code:\n%s", strings.Join(v, "\n"))
	}
}
