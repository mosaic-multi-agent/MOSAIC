package domain

// builtinTools are each harness's own built-in tool names. Only the claude-code
// and opencode lists are captured; the other harnesses have none, so no tool name
// is judged unknown for them.
func builtinTools() map[Harness][]string {
	return map[Harness][]string{
		HarnessClaudeCode: {
			"Bash", "BashOutput", "KillShell", "Read", "Write", "Edit", "MultiEdit",
			"NotebookEdit", "Glob", "Grep", "Agent", "Task", "TodoWrite", "WebFetch",
			"WebSearch", "AskUserQuestion", "SubagentHandback", "ExitPlanMode",
			"Skill", "SlashCommand", "ToolSearch",
		},
		HarnessOpenCode: {
			"bash", "read", "write", "edit", "patch", "apply_patch", "glob", "grep",
			"list", "todowrite", "todoread", "webfetch", "task", "question", "skill",
		},
	}
}

// HarnessBuiltinTools lists each harness's own built-in tool names, used only
// for the missing-contact-tools hint. Fixed in code, not configurable. An
// unknown harness ("") gets the union of all lists; a harness without a captured
// list gets none.
func HarnessBuiltinTools(h Harness) []string {
	all := builtinTools()
	if h != "" {
		return all[h]
	}
	var union []string
	for _, list := range all {
		union = append(union, list...)
	}
	return union
}
