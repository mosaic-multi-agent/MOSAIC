package main

// notice_routing_binary_test.go runs the built binary to verify where notices go. Promoting a
// Codex source agent raises a notice (the Codex file does not record its tools), and the notice
// must never land on stdout next to the result document. The wiring under test is shared by every
// subcommand, so one subcommand with several --output spellings guards it for all.

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

const codexToolsNoticeText = "source harness does not record tool information"

// codexSourceToml is a Codex agent file that is eligible for promote.
const codexSourceToml = "# mosaic_transform_version: 2.1.0\n" +
	"# mosaic_version: 1.0.0\n" +
	"\n" +
	"name = \"my-codex-agent\"\n" +
	"description = \"A Codex agent for notice routing tests\"\n" +
	"sandbox_mode = \"read-only\"\n" +
	"developer_instructions = \"\"\"\n" +
	"<Identity type=\"core\">\n" +
	"You are My Codex Agent.\n" +
	"</Identity>\n" +
	"\"\"\"\n"

// newCodexPromoteFixture returns a temporary MOSAIC root and the path of a temporary Codex
// source agent file.
func newCodexPromoteFixture(t *testing.T) (mosaicRoot, sourceFile string) {
	t.Helper()
	mosaicRoot = newStackMosaicRoot(t)
	writeHarnessInjectionFiles(t, filepath.Join(mosaicRoot, "Catalog"),
		catalogpaths.HarnessDirNameCodex, "codex", "Fixture constraint.", "1.0.0", "Fixture constraint.")
	src := t.TempDir()
	writeStackFile(t, src, "my-codex-agent.toml", []byte(codexSourceToml))
	return mosaicRoot, filepath.Join(src, "my-codex-agent.toml")
}

// runCodexPromote promotes the fixture's Codex agent with the extra args (the --output spelling).
func runCodexPromote(t *testing.T, extra ...string) binaryRun {
	t.Helper()
	root, file := newCodexPromoteFixture(t)
	args := append([]string{"promote", "--file", file, "--harness", "codex", "--category", "TestCategory", "--mosaic-root", root}, extra...)
	run := runDeployBinary(t, args...)
	if run.ExitCode != 0 {
		t.Fatalf("promote of the Codex fixture failed (exit %d)\nstderr: %s\nstdout: %s", run.ExitCode, run.Stderr, run.Stdout)
	}
	return run
}

// requireSingleJSONDocument fails unless stdout is exactly one JSON document with no other text.
func requireSingleJSONDocument(t *testing.T, run binaryRun) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(run.Stdout))
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout does not start with a JSON document: %v\nstdout: %q\nstderr: %s", err, run.Stdout, run.Stderr)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		t.Errorf("stdout carries more than one JSON document or trailing text\nstdout: %q", run.Stdout)
	}
	if rest := strings.TrimSpace(run.Stdout[dec.InputOffset():]); rest != "" {
		t.Errorf("stdout carries text after the JSON document: %q", rest)
	}
}

func TestBinary_PromoteCodexSource_JsonOutput_StdoutIsExactlyOneJSONDocument(t *testing.T) {
	spellings := map[string][]string{
		"space separated": {"--output", "json"},
		"equals form":     {"--output=json"},
		"upper case":      {"--output", "JSON"},
	}
	for name, extra := range spellings {
		t.Run(name, func(t *testing.T) {
			run := runCodexPromote(t, extra...)

			requireSingleJSONDocument(t, run)
			if !strings.Contains(run.Stderr, codexToolsNoticeText) && !strings.Contains(run.Stdout, codexToolsNoticeText) {
				t.Errorf("the notice text is on neither stream; it must stay visible\nstderr: %q\nstdout: %q", run.Stderr, run.Stdout)
			}
		})
	}
}

func TestBinary_PromoteCodexSource_HumanOutput_NoticeStaysVisible(t *testing.T) {
	run := runCodexPromote(t)

	if !strings.Contains(run.Stdout, codexToolsNoticeText) && !strings.Contains(run.Stderr, codexToolsNoticeText) {
		t.Errorf("the notice text is on neither stream\nstderr: %q\nstdout: %q", run.Stderr, run.Stdout)
	}
}
