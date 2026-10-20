package identity

import (
	"fmt"
	"os"
	"regexp"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Read reads and validates the identity of the catalog rooted at catalogRoot. Every error wraps
// exactly one sentinel of this package and names the catalog root and the identity file path.
//
// version is kept as the scalar's source text: the YAML value is read from the syntax tree, not
// decoded into a number, so 1.10 stays "1.10".
func Read(catalogRoot string) (Identity, error) {
	path := catalogpaths.IdentityFile(catalogRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return Identity{}, fail(ErrIdentityMissing, catalogRoot, path, err.Error())
	}
	top, err := topLevelMap(data)
	if err != nil {
		return Identity{}, fail(ErrIdentityInvalid, catalogRoot, path, err.Error())
	}

	var id Identity
	fields := []struct {
		key string
		dst *string
	}{{"id", &id.ID}, {"name", &id.Name}, {"version", &id.Version}}
	for _, f := range fields {
		text, present, err := scalarText(top[f.key])
		if err != nil {
			return Identity{}, fail(ErrIdentityInvalid, catalogRoot, path, fmt.Sprintf("field %q: %v", f.key, err))
		}
		if !present || text == "" {
			return Identity{}, fail(ErrIdentityFieldMissing, catalogRoot, path, fmt.Sprintf("field %q", f.key))
		}
		*f.dst = text
	}
	if !idPattern.MatchString(id.ID) {
		return Identity{}, fail(ErrIdentityInvalid, catalogRoot, path,
			fmt.Sprintf("field \"id\" %q must match %s", id.ID, idPattern))
	}

	overrides, err := parseOverrides(top["overrides"])
	if err != nil {
		return Identity{}, fail(ErrOverrideInvalid, catalogRoot, path, err.Error())
	}
	id.Overrides = overrides
	return id, nil
}

func fail(sentinel error, catalogRoot, path, detail string) error {
	return fmt.Errorf("%w: catalog %s (%s): %s", sentinel, catalogRoot, path, detail)
}

// topLevelMap parses data and returns the top-level mapping's values by key.
func topLevelMap(data []byte) (map[string]ast.Node, error) {
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if len(file.Docs) == 0 || file.Docs[0].Body == nil {
		return nil, fmt.Errorf("file is empty")
	}
	return mappingEntries(file.Docs[0].Body)
}

func mappingEntries(node ast.Node) (map[string]ast.Node, error) {
	out := map[string]ast.Node{}
	switch n := node.(type) {
	case *ast.MappingNode:
		for _, mv := range n.Values {
			out[mv.Key.GetToken().Value] = mv.Value
		}
	case *ast.MappingValueNode:
		out[n.Key.GetToken().Value] = n.Value
	default:
		return nil, fmt.Errorf("top level must be a mapping")
	}
	return out, nil
}

// scalarText returns the source text of a scalar node. present is false for an absent or null
// value; a list or mapping value is an error.
func scalarText(node ast.Node) (text string, present bool, err error) {
	if node == nil {
		return "", false, nil
	}
	switch node.(type) {
	case *ast.NullNode:
		return "", false, nil
	case *ast.SequenceNode, *ast.MappingNode, *ast.MappingValueNode:
		return "", false, fmt.Errorf("must be a scalar value")
	}
	return node.GetToken().Value, true, nil
}
