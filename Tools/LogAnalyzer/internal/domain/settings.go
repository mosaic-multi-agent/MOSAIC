package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// AnalysisSettings holds the user-tunable analysis settings read from the
// settings file. A tool name is an exact name or a pattern where '*' matches any
// run of characters (including none); every other character is literal and
// matching is case-sensitive over the whole name.
type AnalysisSettings struct {
	// WriteTools and UserInteractionTools map a harness to its name list.
	WriteTools           map[Harness][]string
	UserInteractionTools map[Harness][]string
	// CompactionDropPercent: a drop of at least this percentage between two
	// consecutive calls is a detected compaction. 1..99; default 30.
	CompactionDropPercent int
}

// DefaultCompactionDropPercent is the built-in detected-compaction threshold.
const DefaultCompactionDropPercent = 30

// ErrInvalidToolName is returned (wrapped) for an empty or malformed tool name.
var ErrInvalidToolName = errors.New("invalid tool name")

// ErrInvalidHarness is returned (wrapped) when the harness is "".
var ErrInvalidHarness = errors.New("invalid harness")

// defaultWriteTools are the built-in write-class tool names per harness. Only
// the claude-code names are verified against real MOSAIC logs.
func defaultWriteTools() map[Harness][]string {
	return map[Harness][]string{
		HarnessClaudeCode: {"Write", "Edit", "MultiEdit", "NotebookEdit"},
		HarnessGHCPCLI:    {"create", "edit"},
		HarnessOpenCode:   {"apply_patch"},
		HarnessVSCodeGHCP: {
			"create_file", "replace_string_in_file", "multi_replace_string_in_file",
			"insert_edit_into_file", "apply_patch", "edit_notebook_file",
			"edit/createFile", "edit/editFiles",
		},
	}
}

// defaultUserInteractionTools are the built-in question-tool names per harness.
func defaultUserInteractionTools() map[Harness][]string {
	return map[Harness][]string{
		HarnessClaudeCode: {"AskUserQuestion"},
		HarnessGHCPCLI:    {"ask_user"},
		HarnessOpenCode:   {"question"},
		HarnessVSCodeGHCP: {"vscode/askQuestions", "askQuestions", "vscode_askQuestions"},
	}
}

// DefaultWriteTools returns the built-in write-class list for h (nil when none).
func DefaultWriteTools(h Harness) []string { return defaultWriteTools()[h] }

// DefaultUserInteractionTools returns the built-in user-interaction list for h
// (nil when none).
func DefaultUserInteractionTools(h Harness) []string { return defaultUserInteractionTools()[h] }

// DefaultAnalysisSettings returns the built-in settings.
func DefaultAnalysisSettings() AnalysisSettings {
	return AnalysisSettings{
		WriteTools:            defaultWriteTools(),
		UserInteractionTools:  defaultUserInteractionTools(),
		CompactionDropPercent: DefaultCompactionDropPercent,
	}
}

// IsWriteTool matches tool against the write-class list for h; h == "" uses the
// union of all harness lists. A tool name "" never matches.
func (s AnalysisSettings) IsWriteTool(h Harness, tool string) bool {
	return matchToolList(s.WriteTools, h, tool)
}

// IsUserInteractionTool matches tool against the user-interaction list for h;
// h == "" uses the union of all harness lists. A tool name "" never matches.
func (s AnalysisSettings) IsUserInteractionTool(h Harness, tool string) bool {
	return matchToolList(s.UserInteractionTools, h, tool)
}

// WithUserInteractionTool returns a copy with name appended to h's list (no-op
// when already present). When h has no list, the copy holds h's default list
// plus name. s is unchanged on error.
func (s AnalysisSettings) WithUserInteractionTool(h Harness, name string) (AnalysisSettings, error) {
	if h == "" {
		return s, fmt.Errorf("%w: harness is empty", ErrInvalidHarness)
	}
	if err := ValidateToolName(name); err != nil {
		return s, err
	}
	out := s
	out.UserInteractionTools = make(map[Harness][]string, len(s.UserInteractionTools)+1)
	for k, v := range s.UserInteractionTools {
		out.UserInteractionTools[k] = append([]string(nil), v...)
	}
	list, ok := out.UserInteractionTools[h]
	if !ok {
		list = append([]string(nil), DefaultUserInteractionTools(h)...)
	}
	for _, existing := range list {
		if existing == name {
			out.UserInteractionTools[h] = list
			return out, nil
		}
	}
	out.UserInteractionTools[h] = append(list, name)
	return out, nil
}

// ValidateToolName rejects "" / whitespace-only names and names containing a
// control character (including newline).
func ValidateToolName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidToolName)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %q contains a control character", ErrInvalidToolName, name)
		}
	}
	return nil
}

func matchToolList(lists map[Harness][]string, h Harness, tool string) bool {
	if tool == "" {
		return false
	}
	if h != "" {
		return matchAny(lists[h], tool)
	}
	for _, list := range lists {
		if matchAny(list, tool) {
			return true
		}
	}
	return false
}

func matchAny(patterns []string, tool string) bool {
	for _, p := range patterns {
		if matchGlob(p, tool) {
			return true
		}
	}
	return false
}

// matchGlob reports whether name matches pattern over the whole string, where
// '*' matches any run of characters and every other character is literal.
func matchGlob(pattern, name string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == name
	}
	if !strings.HasPrefix(name, parts[0]) {
		return false
	}
	name = name[len(parts[0]):]
	last := parts[len(parts)-1]
	for _, mid := range parts[1 : len(parts)-1] {
		i := strings.Index(name, mid)
		if i < 0 {
			return false
		}
		name = name[i+len(mid):]
	}
	return strings.HasSuffix(name, last)
}
