package main

// codex_fake_test.go holds the fake Codex CLI of the end-to-end tests. The test
// binary doubles as the fake `codex` executable when GO_WANT_FAKE_CODEX=1 is
// set: it records every invocation (argv and stdin) as one JSON line in the file
// named by GO_FAKE_CODEX_LOG, then answers like a Codex turn would, as JSON
// Lines, with the reply the Runner expects for the request it was sent.

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

const (
	fakeCodexEnv    = "GO_WANT_FAKE_CODEX"
	fakeCodexLogEnv = "GO_FAKE_CODEX_LOG"
)

// fakeCodexCall is one recorded invocation of the fake Codex CLI.
type fakeCodexCall struct {
	Args  []string `json:"args"`
	Stdin string   `json:"stdin"`
}

// maybeRunFakeCodex runs the fake Codex CLI and exits when the test binary was
// started as one; otherwise it returns immediately.
func maybeRunFakeCodex() {
	if os.Getenv(fakeCodexEnv) != "1" {
		return
	}
	stdin, _ := io.ReadAll(os.Stdin)
	recordFakeCodexCall(fakeCodexCall{Args: os.Args[1:], Stdin: string(stdin)})
	emitFakeCodexTurn(fakeCodexReply(string(stdin)))
	os.Exit(0)
}

func recordFakeCodexCall(call fakeCodexCall) {
	path := os.Getenv(fakeCodexLogEnv)
	if path == "" {
		return
	}
	line, _ := json.Marshal(call)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(line, '\n')) //nolint:errcheck
}

// fakeCodexReply returns the agent message for the request in stdin: a protocol
// response for a subagent invocation, an empty advice for the run-start
// consultation and a stop for any routing consultation.
func fakeCodexReply(stdin string) string {
	req := fakeCodexRequest(stdin)
	if id, ok := req["agent_instance_id"].(string); ok {
		reply, _ := json.Marshal(map[string]string{
			"agent_instance_id": id,
			"status_code":       "SUCCESS",
			"status_message":    "done " + id,
		})
		return string(reply)
	}
	if req["context"] == "pre_consultation" {
		return `{}`
	}
	return `{"action":"stop","reason":"nothing more to do"}`
}

// fakeCodexRequest decodes the JSON request that follows the agent
// instructions in stdin; it is empty when stdin carries none.
func fakeCodexRequest(stdin string) map[string]any {
	req := map[string]any{}
	for i := 0; i < len(stdin); i++ {
		if stdin[i] == '{' && (i == 0 || stdin[i-1] == '\n') {
			_ = json.NewDecoder(strings.NewReader(stdin[i:])).Decode(&req)
			return req
		}
	}
	return req
}

func emitFakeCodexTurn(text string) {
	msg, _ := json.Marshal(text)
	lines := []string{
		`{"type":"thread.started","thread_id":"t1"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":` + string(msg) + `}}`,
		`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}`,
	}
	os.Stdout.WriteString(strings.Join(lines, "\n") + "\n") //nolint:errcheck
}
