// Package agentfile tells the Runner where a harness's agent files live and
// how they are named and encoded.
package agentfile

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	commonharness "mosaic-common/harness"
)

const formatMarkdown = "markdown"

// Format is the Runner's per-harness agent file data, from the shared catalog
// or a test-only registration. The zero value is legacy Markdown (accepts
// .agent.md and .md) with no agents directory.
type Format struct {
	AgentsDir string // relative agents directory; "" when unknown
	FormatID  string // formatid value; "" means Markdown
	Extension string // catalog agent extension incl. dot; "" allowed for Markdown
}

var (
	testMu         sync.Mutex
	testFormatData = map[string]Format{}
)

// For returns the Format of harnessID's agent files. ok reports whether the
// harness has format data (a catalog entry, or a test-only registration).
// When ok is false the returned Format is the zero Format.
func For(harnessID string) (f Format, ok bool) {
	if entry, found := commonharness.LookupCLIHarness(harnessID); found {
		return Format{
			AgentsDir: entry.AgentsDir,
			FormatID:  entry.AgentFormatID,
			Extension: entry.AgentExtension,
		}, true
	}
	testMu.Lock()
	defer testMu.Unlock()
	if reg, found := testFormatData[harnessID]; found {
		return reg, true
	}
	return Format{}, false
}

// RegisterTestOnly gives harnessID format data for the current process; the
// returned function removes it. Tests only; production code never calls it.
// Panics when harnessID is a catalog harness or is already test-registered.
func RegisterTestOnly(harnessID string, f Format) (unregister func()) {
	if _, found := commonharness.LookupCLIHarness(harnessID); found {
		panic(fmt.Sprintf("agentfile: harness %q has a catalog entry; the catalog always wins", harnessID))
	}
	testMu.Lock()
	defer testMu.Unlock()
	if _, found := testFormatData[harnessID]; found {
		panic(fmt.Sprintf("agentfile: harness %q is already test-registered", harnessID))
	}
	testFormatData[harnessID] = f
	return func() {
		testMu.Lock()
		defer testMu.Unlock()
		delete(testFormatData, harnessID)
	}
}

func (f Format) isMarkdown() bool {
	return f.FormatID == "" || f.FormatID == formatMarkdown
}

// Extensions returns the agent file extensions f accepts, longest first.
// Markdown format: ".agent.md", ".md", plus f.Extension if it is neither.
// Other formats: exactly f.Extension.
func (f Format) Extensions() []string {
	var exts []string
	if f.isMarkdown() {
		exts = []string{".agent.md", ".md"}
		if f.Extension != "" && f.Extension != ".agent.md" && f.Extension != ".md" {
			exts = append(exts, f.Extension)
		}
	} else {
		exts = []string{f.Extension}
	}
	sort.SliceStable(exts, func(i, j int) bool { return len(exts[i]) > len(exts[j]) })
	return exts
}

// Stem returns the agent identifier for filename: filename minus the longest
// accepted extension, matched case-insensitively. "" when no accepted
// extension matches or the remaining stem is empty.
func (f Format) Stem(filename string) string {
	lower := strings.ToLower(filename)
	for _, ext := range f.Extensions() {
		if ext == "" {
			continue
		}
		if strings.HasSuffix(lower, strings.ToLower(ext)) {
			return filename[:len(filename)-len(ext)]
		}
	}
	return ""
}
