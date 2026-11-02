package otelingest

import "strings"

// Call kinds written to usage_record.call_kind.
const (
	callKindMain       = "main"
	callKindCompaction = "compaction"
	callKindAuxiliary  = "auxiliary"
)

// callKindOf maps a VS Code chat span's purpose name to a call kind. Leading retry-<letters>- prefixes are
// ignored; an unknown name gives "" (no call_kind). The comparison is case-sensitive.
func callKindOf(label string) string {
	name := stripRetryPrefixes(label)
	switch {
	case name == "panel/editAgent", name == "tool/runSubagent", strings.HasPrefix(name, "tool/runSubagent-"):
		return callKindMain
	case strings.HasPrefix(name, "summarizeConversationHistory"):
		return callKindCompaction
	case name == "backgroundTodoAgent", name == "title", name == "progressMessages",
		strings.HasPrefix(name, "copilotLanguageModelWrapper/"):
		return callKindAuxiliary
	}
	return ""
}

// stripRetryPrefixes removes every leading "retry-<letters>-" prefix.
func stripRetryPrefixes(name string) string {
	for strings.HasPrefix(name, "retry-") {
		rest := name[len("retry-"):]
		i := 0
		for i < len(rest) && (rest[i] >= 'a' && rest[i] <= 'z' || rest[i] >= 'A' && rest[i] <= 'Z') {
			i++
		}
		if i == 0 || i >= len(rest) || rest[i] != '-' {
			break
		}
		name = rest[i+1:]
	}
	return name
}
