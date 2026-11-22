package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

const toolDestinationsKey = "tool_destinations"

// trivialToolDestinationsLine matches the tool_destinations key line when its value is
// absent, null or an empty flow collection (optionally followed by a comment).
var trivialToolDestinationsLine = regexp.MustCompile(`^tool_destinations[ \t]*:[ \t]*(?:\{\}|\[\])?[ \t]*(?:#.*)?$`)

// emptyFlowValue matches the empty flow collection that follows a key's colon.
var emptyFlowValue = regexp.MustCompile(`:[ \t]*(?:\{\}|\[\])`)

// PutToolDestination is declared by UserConfigStore; see its documentation.
func (s *userConfigStore) PutToolDestination(harnessID string, mapping domain.ToolMapping) error {
	if err := validatePutMapping(harnessID, mapping); err != nil {
		return err
	}
	existing, err := os.ReadFile(s.filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read existing user config: %w", err)
	}
	out, err := putToolDestinationText(existing, harnessID, mapping)
	if err != nil {
		return fmt.Errorf("%s: %w", s.filePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("create user config directory: %w", err)
	}
	if err := os.WriteFile(s.filePath, out, 0o644); err != nil {
		return fmt.Errorf("write user config: %w", err)
	}
	return nil
}

// validatePutMapping applies the tool_destinations validation rules to a single mapping.
func validatePutMapping(harnessID string, mapping domain.ToolMapping) error {
	if harnessID == "" {
		return errors.New("tool_destinations: harness id is empty")
	}
	if mapping.Generic == "" {
		return errors.New("tool_destinations: generic is empty")
	}
	errs := descriptor.ValidateToolMappings([]domain.ToolMapping{mapping}, toolDestinationsKey+"."+harnessID)
	if len(errs) > 0 {
		return fmt.Errorf("invalid tool_destinations entry: %s: %s", errs[0].Field, errs[0].Message)
	}
	return nil
}

// putToolDestinationText returns existing with the entry for mapping added or replaced under
// harnessID. existing may be empty (missing file). Line endings of existing are kept.
func putToolDestinationText(existing []byte, harnessID string, mapping domain.ToolMapping) ([]byte, error) {
	if len(existing) > 0 {
		if _, err := parser.ParseBytes(existing, parser.ParseComments); err != nil {
			return nil, fmt.Errorf("parse existing user config: %w", err)
		}
	}
	text := string(existing)
	crlf := strings.Contains(text, "\r\n")
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	lines, err := putEntryLines(splitConfigLines(text), harnessID, toWireToolMappings([]domain.ToolMapping{mapping})[0])
	if err != nil {
		return nil, err
	}
	out := strings.Join(lines, "\n")
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	if _, err := parser.ParseBytes([]byte(out), parser.ParseComments); err != nil {
		return nil, fmt.Errorf("edited user config does not parse: %w", err)
	}
	return []byte(out), nil
}

// putEntryLines applies the entry edit to the document lines.
func putEntryLines(lines []string, harnessID string, w wireToolMapping) ([]string, error) {
	start, end := findKeyBlock(lines, toolDestinationsKey)
	if start < 0 {
		block := []string{toolDestinationsKey + ":", "  " + yamlScalar(harnessID) + ":"}
		return appendKeyBlock(lines, append(block, entryLines(w, 4)...)), nil
	}
	if !trivialToolDestinationsLine.MatchString(lines[start]) {
		return rewriteWholeBlock(lines, start, end, harnessID, w)
	}
	if end == start+1 {
		keyLine := emptyFlowValue.ReplaceAllString(lines[start], ":")
		repl := []string{keyLine, "  " + yamlScalar(harnessID) + ":"}
		return spliceLines(lines, start, end, append(repl, entryLines(w, 4)...)), nil
	}
	return putIntoBlock(lines, start, end, harnessID, w)
}

// rewriteWholeBlock handles a tool_destinations value written in flow style: the whole value
// is decoded, edited and re-rendered in block style.
func rewriteWholeBlock(lines []string, start, end int, harnessID string, w wireToolMapping) ([]string, error) {
	var doc struct {
		Dest map[string][]wireToolMapping `yaml:"tool_destinations"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[start:end], "\n")), &doc); err != nil {
		return nil, fmt.Errorf("read existing %s: %w", toolDestinationsKey, err)
	}
	if doc.Dest == nil {
		doc.Dest = map[string][]wireToolMapping{}
	}
	doc.Dest[harnessID] = upsertWireMapping(doc.Dest[harnessID], w)
	block, err := marshalKeyBlock(toolDestinationsKey, doc.Dest)
	if err != nil {
		return nil, err
	}
	return spliceLines(lines, start, end, block), nil
}

// harnessKey returns the mapping key of a harness line, or "" when l is not a key line.
func harnessKey(l string) string {
	t := strings.TrimSpace(l)
	idx := strings.Index(t, ":")
	if idx <= 0 || strings.HasPrefix(t, "-") {
		return ""
	}
	return strings.Trim(t[:idx], `"'`)
}

// putIntoBlock edits a tool_destinations block in block style; lines[start+1:end] hold its
// harness entries.
func putIntoBlock(lines []string, start, end int, harnessID string, w wireToolMapping) ([]string, error) {
	first := -1
	for i := start + 1; i < end; i++ {
		if isContentLine(lines[i]) {
			first = i
			break
		}
	}
	if first < 0 {
		return nil, fmt.Errorf("%s block has no entries", toolDestinationsKey)
	}
	childIndent := leadingSpaces(lines[first])
	var heads []int
	for i := first; i < end; i++ {
		if isContentLine(lines[i]) && leadingSpaces(lines[i]) == childIndent && harnessKey(lines[i]) != "" {
			heads = append(heads, i)
		}
	}
	for n, h := range heads {
		if harnessKey(lines[h]) != harnessID {
			continue
		}
		stop := end
		if n+1 < len(heads) {
			stop = heads[n+1]
		}
		return putIntoHarness(lines, h, lastContentIndex(lines, h, stop)+1, childIndent, w)
	}
	header := strings.Repeat(" ", childIndent) + yamlScalar(harnessID) + ":"
	repl := append([]string{header}, entryLines(w, childIndent+2)...)
	return spliceLines(lines, end, end, repl), nil
}

// putIntoHarness edits one harness's list: lines[head:stop] are its key line and value lines.
func putIntoHarness(lines []string, head, stop, childIndent int, w wireToolMapping) ([]string, error) {
	colon := strings.Index(lines[head], ":")
	inline := strings.TrimSpace(lines[head][colon+1:])
	if inline != "" && !strings.HasPrefix(inline, "#") {
		return rewriteFlowHarness(lines, head, stop, colon, childIndent, w)
	}
	if stop == head+1 {
		return spliceLines(lines, stop, stop, entryLines(w, childIndent+2)), nil
	}
	itemIndent := leadingSpaces(lines[head+1+firstContentOffset(lines[head+1:stop])])
	var dashes []int
	for i := head + 1; i < stop; i++ {
		if isContentLine(lines[i]) && leadingSpaces(lines[i]) == itemIndent && strings.HasPrefix(strings.TrimSpace(lines[i]), "-") {
			dashes = append(dashes, i)
		}
	}
	for n, d := range dashes {
		limit := stop
		if n+1 < len(dashes) {
			limit = dashes[n+1]
		}
		entryEnd := lastContentIndex(lines, d, limit) + 1
		var parsed []wireToolMapping
		if err := yaml.Unmarshal([]byte(dedent(lines[d:entryEnd], itemIndent)), &parsed); err != nil || len(parsed) != 1 {
			continue
		}
		if parsed[0].Generic == w.Generic {
			return spliceLines(lines, d, entryEnd, entryLines(w, itemIndent)), nil
		}
	}
	return spliceLines(lines, stop, stop, entryLines(w, itemIndent)), nil
}

// firstContentOffset returns the index of the first content line in lines (0 when none).
func firstContentOffset(lines []string) int {
	for i, l := range lines {
		if isContentLine(l) {
			return i
		}
	}
	return 0
}

// rewriteFlowHarness re-renders a harness whose list is written in flow style in block
// style, with w added or replacing its generic.
func rewriteFlowHarness(lines []string, head, stop, colon, childIndent int, w wireToolMapping) ([]string, error) {
	value := lines[head][colon+1:] + "\n" + strings.Join(lines[head+1:stop], "\n")
	var list []wireToolMapping
	if err := yaml.Unmarshal([]byte(value), &list); err != nil {
		return nil, fmt.Errorf("read existing %s entries: %w", toolDestinationsKey, err)
	}
	repl := []string{strings.TrimRight(lines[head][:colon+1], " \t")}
	for _, m := range upsertWireMapping(list, w) {
		repl = append(repl, entryLines(m, childIndent+2)...)
	}
	return spliceLines(lines, head, stop, repl), nil
}
