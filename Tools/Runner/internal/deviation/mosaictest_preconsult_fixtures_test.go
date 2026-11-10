package deviation

// Pre-consultation check for the MosaicTest routing fixtures shipped in the
// Runner's own TestCatalog. The stub orchestrator answers the run-start
// pre-consultation from the "## Pre-Consultation" section of the fixture: the
// bare token "none", advice blocks, or a stop with a reason. Each section must
// be turned into the reply the stub returns and be accepted by the Runner's
// pre-consultation parser as the same advice or the same stop.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// preConsultSection is the parsed "## Pre-Consultation" section of a fixture.
type preConsultSection struct {
	none            bool
	stop            bool
	reason          string
	taskDescription string
	constraints     string
	hasTask         bool
	hasConstraints  bool
}

func TestMosaicTestRoutingFixtures_PreConsultationSectionIsAcceptedByTheParser(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(mosaicTestWorkflowsRoot, "Fixtures", "*", "MosaicTestRouting.md"))
	if err != nil {
		t.Fatalf("glob routing fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no MosaicTestRouting.md fixture found")
	}
	sort.Strings(paths)

	for _, path := range paths {
		name := filepath.Base(filepath.Dir(path))
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read routing fixture: %v", err)
			}
			section, ok := parsePreConsultSection(string(content))
			if !ok {
				t.Fatal("fixture has no recognisable ## Pre-Consultation section")
			}
			assertParserAcceptsSection(t, section)
		})
	}
}

func TestMosaicTestRoutingFixtures_PreConsultStopFixtureDeclaresAStopWithReason(t *testing.T) {
	path := filepath.Join(mosaicTestWorkflowsRoot, "Fixtures", "preconsult-stop", "MosaicTestRouting.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read routing fixture: %v", err)
	}

	section, ok := parsePreConsultSection(string(content))

	if !ok || !section.stop {
		t.Fatalf("preconsult-stop fixture must declare the stop form, got %+v", section)
	}
	if !strings.Contains(section.reason, "PRECONSULT-STOP-MARKER") {
		t.Errorf("stop reason must carry the marker the expectation asserts, got %q", section.reason)
	}
}

// assertParserAcceptsSection renders the stub reply for the section, feeds it
// through the Runner's parser and compares the result with the section.
func assertParserAcceptsSection(t *testing.T, section preConsultSection) {
	t.Helper()
	advice, err := parsePreConsultReply(buildPreConsultReply(section))
	if err != nil {
		t.Fatalf("pre-consultation reply refused by the Runner parser: %v", err)
	}
	if section.stop {
		if advice.Stop == nil {
			t.Fatal("stop form was accepted as advice, want a stop")
		}
		if advice.Stop.Reason != section.reason {
			t.Errorf("stop reason = %q, want %q", advice.Stop.Reason, section.reason)
		}
		return
	}
	if advice.Stop != nil {
		t.Fatalf("advice form was accepted as a stop: %q", advice.Stop.Reason)
	}
	if advice.TaskDescription != section.taskDescription || advice.Constraints != section.constraints {
		t.Errorf("advice = %+v, want task %q constraints %q", advice, section.taskDescription, section.constraints)
	}
}

// buildPreConsultReply renders the JSON object the stub returns for a section.
func buildPreConsultReply(section preConsultSection) []byte {
	reply := map[string]any{}
	switch {
	case section.stop:
		reply["action"] = "stop"
		reply["reason"] = section.reason
	case section.none:
	default:
		if section.hasTask {
			reply["task_description"] = section.taskDescription
		}
		if section.hasConstraints {
			reply["constraints"] = section.constraints
		}
	}
	return mustMarshal(reply)
}

// parsePreConsultSection reads the "## Pre-Consultation" section: the bare
// token none, "task_description:" and "constraints:" key lines with
// tilde-fenced values, or a bare stop line followed by a "reason:" key line
// with a tilde-fenced value.
func parsePreConsultSection(content string) (preConsultSection, bool) {
	var section preConsultSection
	found, inFence := false, false
	key := ""
	var fence []string

	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if inFence {
			if line == "~~~" {
				inFence = false
				setPreConsultFence(&section, key, strings.Join(fence, "\n"))
				fence = nil
				continue
			}
			fence = append(fence, line)
			continue
		}
		switch {
		case strings.TrimSpace(line) == "## Pre-Consultation":
			found = true
		case found && strings.HasPrefix(line, "## "):
			return section, section.none || section.stop || section.hasTask || section.hasConstraints
		case !found:
		case line == "~~~":
			inFence = true
		case strings.TrimSpace(line) == "none":
			section.none = true
		case strings.TrimSpace(line) == "stop":
			section.stop = true
		case strings.HasSuffix(strings.TrimSpace(line), ":"):
			key = strings.TrimSuffix(strings.TrimSpace(line), ":")
		}
	}
	return section, found && (section.none || section.stop || section.hasTask || section.hasConstraints)
}

func setPreConsultFence(section *preConsultSection, key, value string) {
	switch key {
	case "task_description":
		section.taskDescription, section.hasTask = value, true
	case "constraints":
		section.constraints, section.hasConstraints = value, true
	case "reason":
		section.reason = value
	}
}
