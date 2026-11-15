package harness_test

// Codex modes of the fake-subprocess helper (see helper_test.go). They emit
// `codex exec --json` event streams. A mode records what it received when asked:
// stdin in GO_HELPER_STDIN_FILE, the working directory in GO_HELPER_CWD_FILE.
//
// Modes:
//   - codex-success: a completed turn whose final agent message is the protocol
//     response; exits zero.
//   - codex-exit1: events without a terminal event, text on stderr; exits 1.
//   - codex-turn-failed-exit1: a turn.failed event; exits 1.
//   - codex-turn-failed-exit0: a turn.failed event; exits zero.
//   - codex-incomplete: events without a terminal event; exits zero.
//   - codex-empty: no output; exits zero.
//   - codex-exit-holds-stdout: a completed turn, then exits zero while a
//     descendant that inherited stdout/stderr keeps running (its PID goes to
//     GO_HELPER_PID_FILE).
//   - tree-exit-holds, tree-exit1-holds: see processkill_helper_test.go.

import (
	"io"
	"os"
	"os/exec"
)

const codexProtocolResp = `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`

// recordInputs writes stdin and the working directory to the files the test
// configured, so a test can assert what the invocation delivered.
func recordInputs() {
	if path := os.Getenv("GO_HELPER_STDIN_FILE"); path != "" {
		data, _ := io.ReadAll(os.Stdin)
		os.WriteFile(path, data, 0644) //nolint:errcheck
	}
	if path := os.Getenv("GO_HELPER_CWD_FILE"); path != "" {
		cwd, _ := os.Getwd()
		os.WriteFile(path, []byte(cwd), 0644) //nolint:errcheck
	}
}

func codexCompletedTurn() {
	writeOut(`{"type":"thread.started","thread_id":"t1"}` + "\n")
	writeOut(`{"type":"turn.started"}` + "\n")
	writeOut(`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":` + jsonQuote(codexProtocolResp) + `}}` + "\n")
	writeOut(`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}` + "\n")
}

// runCodexHelper handles the Codex helper modes; it returns normally for modes
// that exit zero.
func runCodexHelper(cmd string) {
	recordInputs()
	switch cmd {
	case "codex-success":
		codexCompletedTurn()

	case "codex-exit1":
		writeOut(`{"type":"thread.started","thread_id":"t1"}` + "\n")
		os.Stderr.WriteString("codex crashed") //nolint:errcheck
		os.Exit(1)

	case "codex-turn-failed-exit1":
		writeOut(`{"type":"turn.started"}` + "\n")
		writeOut(`{"type":"turn.failed","error":{"message":"simulated model failure"}}` + "\n")
		os.Exit(1)

	case "codex-turn-failed-exit0":
		writeOut(`{"type":"turn.started"}` + "\n")
		writeOut(`{"type":"turn.failed","error":{"message":"simulated model failure"}}` + "\n")

	case "codex-incomplete":
		writeOut(`{"type":"thread.started","thread_id":"t1"}` + "\n")
		writeOut(`{"type":"turn.started"}` + "\n")

	case "codex-empty":
		// No output.

	case "codex-exit-holds-stdout":
		gc := exec.Command(os.Args[0])
		gc.Env = append(os.Environ(), "GO_HELPER_CMD=tree-grandchild")
		gc.Stdout = os.Stdout
		gc.Stderr = os.Stderr
		if err := gc.Start(); err != nil {
			os.Exit(3)
		}
		writePidFile(gc.Process.Pid)
		codexCompletedTurn()
	}
}
