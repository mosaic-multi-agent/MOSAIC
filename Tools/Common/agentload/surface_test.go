package agentload_test

// surface_test.go guards that the loader's exported surface names none of the
// write-side concepts: create/update, refresh mode, prior bytes, write reports.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

func TestSurface_ExportsNoWriteSideConcepts(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"Encode", "Create", "Update", "Refresh", "Prior", "Report", "Operation", "ArtifactContext", "Write"}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || !id.IsExported() {
					return true
				}
				for _, bad := range forbidden {
					if strings.Contains(id.Name, bad) {
						t.Errorf("exported identifier %q names a write-side concept (%s)", id.Name, bad)
					}
				}
				return true
			})
		}
	}
}
