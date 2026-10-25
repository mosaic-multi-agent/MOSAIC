// Package defmodel reads the model field of a deployed agent definition: a
// Markdown file whose YAML frontmatter carries the model under the top-level
// key "model", which is where the deployment tool writes it for every
// built-in harness.
package defmodel

import (
	"bytes"
	"fmt"
	"os"

	goyaml "github.com/goccy/go-yaml"

	"mosaic-agent-test/internal/domain"
)

const modelKey = "model"

// Read parses the frontmatter of the definition at path and returns its
// top-level model field. A nested "model" key is not the field. The value is
// the parsed scalar (quotes and comments removed, no other normalisation).
// An error is returned only when the file cannot be read or its frontmatter
// cannot be parsed.
func Read(path string) (domain.DefinitionModelField, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return domain.DefinitionModelField{}, fmt.Errorf("reading definition: %w", err)
	}
	front, ok := frontmatter(raw)
	if !ok {
		return domain.DefinitionModelField{}, nil
	}
	var doc map[string]any
	if err := goyaml.Unmarshal(front, &doc); err != nil {
		return domain.DefinitionModelField{}, fmt.Errorf("parsing frontmatter of %s: %w", path, err)
	}
	v, present := doc[modelKey]
	if !present {
		return domain.DefinitionModelField{}, nil
	}
	if v == nil {
		return domain.DefinitionModelField{Present: true}, nil
	}
	return domain.DefinitionModelField{Present: true, Value: fmt.Sprint(v)}, nil
}

// frontmatter returns the text between the opening and closing "---" lines.
func frontmatter(raw []byte) ([]byte, bool) {
	text := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	text = bytes.TrimPrefix(text, []byte("\xef\xbb\xbf"))
	if !bytes.HasPrefix(text, []byte("---\n")) {
		return nil, false
	}
	rest := text[len("---\n"):]
	if bytes.HasPrefix(rest, []byte("---")) {
		return nil, true
	}
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return nil, false
	}
	return rest[:end+1], true
}
