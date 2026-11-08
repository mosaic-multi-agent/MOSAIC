package main

// wiring_forcestop_entry_test.go verifies, at the syntax level, that the
// interactive entry point creates the shared force-stop request and hands it to
// the wiring seam. The entry point cannot be run in a test (it starts the
// terminal program and exits the process), so the construction is checked on
// the parsed source of tui_mode.go.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// runTUIModeBody parses tui_mode.go and returns the body of runTUIMode.
func runTUIModeBody(t *testing.T) *ast.BlockStmt {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "tui_mode.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing tui_mode.go: %v", err)
	}
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "runTUIMode" && fn.Body != nil {
			return fn.Body
		}
	}
	t.Fatal("runTUIMode not found in tui_mode.go")
	return nil
}

func TestRunTUIMode_CreatesTheForceStopRequestAndPassesItToTheWiring(t *testing.T) {
	body := runTUIModeBody(t)

	var constructed bool
	var passed ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewForceStopSignal" {
				constructed = true
			}
		case *ast.CompositeLit:
			if id, ok := v.Type.(*ast.Ident); ok && id.Name == "interactiveWiringInput" {
				for _, el := range v.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "ForceStop" {
						passed = kv.Value
					}
				}
			}
		}
		return true
	})

	if !constructed {
		t.Error("runTUIMode never calls NewForceStopSignal: the TUI would have no shared force-stop request")
	}
	if passed == nil {
		t.Fatal("runTUIMode does not set ForceStop on the wiring input: the control would act on nothing")
	}
	if id, ok := passed.(*ast.Ident); ok && id.Name == "nil" {
		t.Error("runTUIMode passes a nil ForceStop to the wiring")
	}
}
