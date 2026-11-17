package harness_test

// OpenCode V2 modes of the fake-subprocess helper (see helper_test.go). Each
// mode imitates what `opencode run --format json --standalone` writes and
// which exit code it ends with, so the V2 spawner can be driven without a real
// opencode binary.
//
//   - v2-success: final protocol JSON text, stop, exit 0.
//   - v2-echo: final text reporting the child's PWD, working directory and
//     stdin, stop, exit 0.
//   - v2-error-agent / v2-error-model / v2-error-other: a stdout error event,
//     exit 1 (V2 puts the reason in the event, not on stderr).
//   - v2-error-exit0: the same error event with exit 0.
//   - v2-usage: help text on stdout and a reason on stderr, exit 1.
//   - v2-exit1-stream: a started stream with no error event, exit 1.
//   - v2-exit130 / v2-exit143: interrupt and terminate, partial stream.
//   - v2-incomplete-wait: stop followed by background_wait, exit 0.

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
)

const (
	v2HelperProtocol  = `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`
	v2HelperStartLine = `{"type":"step_start","sessionID":"ses_h","part":{"type":"step-start"}}` + "\n"
)

func helperTextLine(text string) string {
	b, _ := json.Marshal(text)
	return `{"type":"text","sessionID":"ses_h","part":{"type":"text","text":` + string(b) + `}}` + "\n"
}

func helperErrorLine(typ, msg string) string {
	b, _ := json.Marshal(msg)
	return `{"type":"error","sessionID":"ses_h","error":{"type":"` + typ + `","message":` + string(b) + `}}` + "\n"
}

func helperFinishLine(reason string) string {
	return `{"type":"step_finish","sessionID":"ses_h","part":{"reason":"` + reason +
		`","cost":0.5,"tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}}}` + "\n"
}

// runV2Helper handles the V2 helper modes; it exits the process for every
// non-zero-exit mode and returns normally for exit 0.
func runV2Helper(cmd string) {
	switch cmd {
	case "v2-success":
		writeOut(v2HelperStartLine + helperTextLine(v2HelperProtocol) + helperFinishLine("stop"))
	case "v2-echo":
		stdin, _ := io.ReadAll(os.Stdin)
		cwd, _ := os.Getwd()
		text := "pwd=" + os.Getenv("PWD") + "\ncwd=" + cwd + "\nstdin=" + string(stdin)
		writeOut(v2HelperStartLine + helperTextLine(text) + helperFinishLine("stop"))
	case "v2-error-agent":
		writeOut(helperErrorLine("unknown", `Agent not found: "no-such-agent"`))
		os.Exit(1)
	case "v2-error-model":
		writeOut(helperErrorLine("provider.no-route", "Model unavailable: openai/no-such-model"))
		os.Exit(1)
	case "v2-error-other":
		writeOut(v2HelperStartLine + helperErrorLine("api.failure", "upstream exploded"))
		os.Exit(1)
	case "v2-error-exit0":
		writeOut(helperErrorLine("unknown", `Agent not found: "no-such-agent"`))
	case "v2-usage":
		writeOut("\nopencode run [message..]\n\nrun opencode with a message\n\nOptions:\n  --agent  agent to use\n")
		os.Stderr.WriteString("Unknown argument: bogus-flag\n") //nolint:errcheck
		os.Exit(1)
	case "v2-exit1-stream":
		writeOut(v2HelperStartLine + helperTextLine("partial"))
		os.Exit(1)
	case "v2-exit130":
		writeOut(v2HelperStartLine)
		os.Exit(130)
	case "v2-exit143":
		writeOut(v2HelperStartLine)
		os.Exit(143)
	case "v2-incomplete-wait":
		writeOut(v2HelperStartLine + helperTextLine("waiting") + helperFinishLine("stop") +
			`{"type":"background_wait","jobs":[{"id":"ses_c","kind":"subagent","title":"t"}]}` + "\n")
	default:
		os.Stderr.WriteString("unknown v2 helper mode " + strconv.Quote(cmd) + "\n") //nolint:errcheck
		os.Exit(2)
	}
}
