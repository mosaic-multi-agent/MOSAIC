package settings

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"mosaic-log-analyzer/internal/domain"
)

// AddUserInteractionTool validates and persists name for harness h.
//
// An absent file is created, including missing parent directories, with the
// commented template ahead of the entry. For an existing file the harness list
// gets name appended (an already-present name is a no-op); a harness without a
// list gets its default list plus name. Other keys, unknown keys and the leading
// comment block are preserved. An unparseable file is left unchanged.
func (s *Store) AddUserInteractionTool(_ context.Context, h domain.Harness, name string) error {
	if h == "" {
		return fmt.Errorf("%w: harness is empty", domain.ErrInvalidHarness)
	}
	if err := domain.ValidateToolName(name); err != nil {
		return err
	}

	var doc yaml.MapSlice
	header := settingsTemplate
	raw, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		header = extractHeaderComments(string(raw))
		if doc, err = parseDocument(raw); err != nil {
			return fmt.Errorf("parse settings file: %w", err)
		}
	case !os.IsNotExist(err):
		return fmt.Errorf("read settings file: %w", err)
	}

	doc, changed := withUserInteractionTool(doc, h, name)
	if !changed {
		return nil
	}
	return s.writeDocument(header, doc)
}

// writeDocument writes the preserved header followed by the marshaled document.
func (s *Store) writeDocument(header string, doc yaml.MapSlice) error {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString(header)
	buf.Write(data)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create settings config directory: %w", err)
	}
	if err := os.WriteFile(s.path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}
	return nil
}

// withUserInteractionTool returns doc with name added to h's user-interaction
// list; changed is false when the name was already present.
func withUserInteractionTool(doc yaml.MapSlice, h domain.Harness, name string) (yaml.MapSlice, bool) {
	sectionIdx := indexOfKey(doc, keyUserInteractionTools)
	var section yaml.MapSlice
	if sectionIdx >= 0 {
		section, _ = doc[sectionIdx].Value.(yaml.MapSlice)
	}

	listIdx := indexOfKey(section, string(h))
	var list []any
	if listIdx >= 0 {
		if seq, ok := section[listIdx].Value.([]any); ok {
			list = seq
		} else {
			listIdx = -1 // not a list: replaced by the default list plus name
		}
	}
	if listIdx < 0 {
		for _, d := range domain.DefaultUserInteractionTools(h) {
			list = append(list, d)
		}
	}
	for _, existing := range list {
		if existing == name {
			return doc, false
		}
	}
	list = append(list, name)

	if listIdx >= 0 {
		section[listIdx].Value = list
	} else if i := indexOfKey(section, string(h)); i >= 0 {
		section[i].Value = list
	} else {
		section = append(section, yaml.MapItem{Key: string(h), Value: list})
	}
	if sectionIdx >= 0 {
		doc[sectionIdx].Value = section
	} else {
		doc = append(doc, yaml.MapItem{Key: keyUserInteractionTools, Value: section})
	}
	return doc, true
}

// indexOfKey returns the position of key in m, or -1.
func indexOfKey(m yaml.MapSlice, key string) int {
	for i, item := range m {
		if fmt.Sprint(item.Key) == key {
			return i
		}
	}
	return -1
}

// extractHeaderComments returns the leading comment and blank lines from a YAML
// file's raw content. Extraction stops at the first non-comment, non-blank line.
// The returned string ends with a newline when non-empty.
func extractHeaderComments(content string) string {
	var sb strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			sb.WriteString(line)
			sb.WriteString("\n")
		} else {
			break
		}
	}
	return sb.String()
}
