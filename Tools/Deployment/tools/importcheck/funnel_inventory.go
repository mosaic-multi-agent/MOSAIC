package main

// funnel_inventory.go implements the read-funnel inventory rule: the non-test files in a
// package that reference a read-funnel identifier must be exactly the files listed in the
// inventory section of the package's inventory file (which is implicitly permitted).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// funnelInventoryRule configures the funnel-inventory rule for one package.
type funnelInventoryRule struct {
	dir           string   // package directory relative to the module root
	inventoryFile string   // base name of the file whose comments hold the inventory
	heading       string   // heading line that starts the inventory section
	identifiers   []string // funnel identifiers whose references are restricted
}

var funnelInventoryRules = []funnelInventoryRule{
	{
		dir:           "internal/app",
		inventoryFile: "deployed_read.go",
		heading:       "FUNNEL-ROUTED READS:",
		identifiers:   []string{"readDeployedArtifact", "decodeDeployedBytes", "funnelReader", "canonicalReader"},
	},
}

var funnelEntryPattern = regexp.MustCompile(`^- ([A-Za-z0-9_]+\.go)(?:[\s(:]|$)`)

func checkFunnelInventory(moduleRoot string, r funnelInventoryRule) (violations []string, errs []string) {
	pkgDir := filepath.Join(moduleRoot, filepath.FromSlash(r.dir))
	inventoryPath := filepath.Join(pkgDir, r.inventoryFile)
	if _, err := os.Stat(inventoryPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("Stat(%q): %v", inventoryPath, err)}
	}
	fset := token.NewFileSet()
	inv, err := parser.ParseFile(fset, inventoryPath, nil, parser.ParseComments)
	if err != nil {
		return nil, []string{fmt.Sprintf("parse %s: %v", inventoryPath, err)}
	}
	listed, found := parseFunnelInventory(inv, r.heading)
	where := r.dir + "/" + r.inventoryFile
	if !found {
		return []string{fmt.Sprintf("%s: no %q section found", where, r.heading)}, nil
	}

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, []string{fmt.Sprintf("ReadDir(%q): %v", pkgDir, err)}
	}
	idSet := map[string]bool{}
	for _, id := range r.identifiers {
		idSet[id] = true
	}
	referenced := map[string]string{} // file -> first identifier referenced
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == r.inventoryFile {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(pkgDir, name), nil, 0)
		if perr != nil {
			errs = append(errs, fmt.Sprintf("parse %s: %v", filepath.Join(pkgDir, name), perr))
			continue
		}
		if id := firstFunnelReference(f, idSet); id != "" {
			referenced[name] = id
		}
	}

	for _, name := range sortedKeys(referenced) {
		if !listed[name] {
			violations = append(violations, fmt.Sprintf("%s/%s: references %s but is not listed in the %q section of %s", r.dir, name, referenced[name], r.heading, where))
		}
	}
	for _, name := range sortedKeys(listed) {
		switch {
		case name == r.inventoryFile:
			violations = append(violations, fmt.Sprintf("%s: lists itself in the %q section; the inventory file is implicitly permitted", where, r.heading))
		case referenced[name] == "":
			violations = append(violations, fmt.Sprintf("%s: lists %s in the %q section, but it does not reference a read-funnel identifier (or does not exist)", where, name, r.heading))
		}
	}
	return violations, errs
}

// parseFunnelInventory returns the file names listed in the section that starts at the
// heading comment line and ends at the next empty comment line.
func parseFunnelInventory(f *ast.File, heading string) (listed map[string]bool, found bool) {
	listed = map[string]bool{}
	for _, cg := range f.Comments {
		inSection := false
		for _, c := range cg.List {
			if !strings.HasPrefix(c.Text, "//") {
				continue
			}
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			switch {
			case !inSection && text == heading:
				inSection, found = true, true
			case inSection && text == "":
				inSection = false
			case inSection:
				if m := funnelEntryPattern.FindStringSubmatch(text); m != nil {
					listed[m[1]] = true
				}
			}
		}
	}
	return listed, found
}

// firstFunnelReference returns the first funnel identifier referenced in code, ignoring the
// names of function and method declarations. It returns "" when there is none.
func firstFunnelReference(f *ast.File, ids map[string]bool) string {
	declNames := map[*ast.Ident]bool{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			declNames[fd.Name] = true
		}
	}
	found := ""
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && found == "" && ids[id.Name] && !declNames[id] {
			found = id.Name
		}
		return found == ""
	})
	return found
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
