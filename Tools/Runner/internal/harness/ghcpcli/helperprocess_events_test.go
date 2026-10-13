package ghcpcli_test

// Fake-CLI scenarios for the GHCP stream events that fail an invocation
// (subagent.deselected, session.error) and for version logging. They are
// registered in helperScenarios (helperprocess_test.go) and dispatched by the
// same GO_HELPER_CMD mechanism as every other scenario.

import "os"

// ghcpVersionedSkillsLine is a session.skills_loaded event whose skill path
// carries the package version, with the backslashes JSON-escaped as in raw
// copilot stdout.
func ghcpVersionedSkillsLine(version string) string {
	return `{"type":"session.skills_loaded","data":{"skills":[{"name":"example-skill","path":"C:\\Users\\someuser\\AppData\\Local\\copilot\\pkg\\win32-x64\\` +
		version + `\\builtin\\example-skill\\SKILL.md"}]}}` + "\n"
}

// ghcpProtocolReplyLine is an assistant.message event carrying a valid
// Communication Protocol response.
func ghcpProtocolReplyLine() string {
	protocolResp := `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`
	return `{"type":"assistant.message","data":{"content":` + opencodeJSONQuote(protocolResp) + `}}` + "\n"
}

// helperScenarioGhcpcliDeselected implements GO_HELPER_CMD=ghcpcli-deselected:
// the stream announces subagent.deselected, then ends in a valid protocol reply
// and a successful result, exactly the shape that passed as success before.
func helperScenarioGhcpcliDeselected() {
	os.Stdout.WriteString(ghcpVersionedSkillsLine("1.0.91"))                 //nolint:errcheck
	os.Stdout.WriteString(`{"type":"subagent.deselected","data":{}}` + "\n") //nolint:errcheck
	os.Stdout.WriteString(ghcpProtocolReplyLine())                           //nolint:errcheck
	os.Stdout.WriteString(`{"type":"result","exitCode":0}` + "\n")           //nolint:errcheck
}

// helperScenarioGhcpcliQuota implements GO_HELPER_CMD=ghcpcli-quota: a failed
// model call reported only in session.error, then a failed result; nothing on
// stderr.
func helperScenarioGhcpcliQuota() {
	os.Stdout.WriteString(ghcpVersionedSkillsLine("1.0.92")) //nolint:errcheck
	os.Stdout.WriteString(`{"type":"session.error","data":{"errorType":"quota","errorCode":"quota_exceeded","statusCode":402,` +
		`"message":"You have exceeded your monthly quota (Request ID: 1)"}}` + "\n") //nolint:errcheck
	os.Stdout.WriteString(`{"type":"result","exitCode":1}` + "\n") //nolint:errcheck
}

// helperScenarioGhcpcliSessionErrorExitZero implements
// GO_HELPER_CMD=ghcpcli-session-error-exit0: session.error followed by a
// result that claims success.
func helperScenarioGhcpcliSessionErrorExitZero() {
	os.Stdout.WriteString(ghcpProtocolReplyLine()) //nolint:errcheck
	os.Stdout.WriteString(`{"type":"session.error","data":{"errorType":"quota","errorCode":"quota_exceeded",` +
		`"message":"You have exceeded your monthly quota"}}` + "\n") //nolint:errcheck
	os.Stdout.WriteString(`{"type":"result","exitCode":0}` + "\n") //nolint:errcheck
}

// helperScenarioGhcpcliVersioned implements GO_HELPER_CMD=ghcpcli-versioned: a
// normal successful stream whose skills event names version 1.0.92.
func helperScenarioGhcpcliVersioned() {
	os.Stdout.WriteString(ghcpVersionedSkillsLine("1.0.92"))       //nolint:errcheck
	os.Stdout.WriteString(ghcpProtocolReplyLine())                 //nolint:errcheck
	os.Stdout.WriteString(`{"type":"result","exitCode":0}` + "\n") //nolint:errcheck
}
