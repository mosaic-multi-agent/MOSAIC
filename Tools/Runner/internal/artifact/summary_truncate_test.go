package artifact_test

// Tests for the Execution Log Summary truncation rule: a summary of 150
// characters (runes) or fewer is kept whole; a longer one keeps its first 50
// and last 100 characters joined by " ... ". The [error:CODE] marker follows
// the cut, and artifacts written with the older 50 + 50 rule are read back
// as written.

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

const truncationJoiner = " ... "

// sequentialText returns n characters that differ from their neighbours, so an
// off-by-one in the head or tail cut changes the result.
func sequentialText(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[i%len(alphabet)])
	}
	return b.String()
}

func TestTruncateSummary_HundredFiftyChars_KeptWhole(t *testing.T) {
	input := sequentialText(150)

	got := artifact.TruncateSummary(input)

	if got != input {
		t.Errorf("150 chars must be kept whole:\n  want: %q\n   got: %q", input, got)
	}
}

func TestTruncateSummary_HundredFiftyOneChars_FirstFiftyAndLastHundred(t *testing.T) {
	input := sequentialText(151)

	got := artifact.TruncateSummary(input)

	want := input[:50] + truncationJoiner + input[151-100:]
	if got != want {
		t.Errorf("151 chars must be first 50 + joiner + last 100:\n  want: %q\n   got: %q", want, got)
	}
}

func TestTruncateSummary_VeryLongInput_ResultHasFixedLength(t *testing.T) {
	input := sequentialText(5000)

	got := artifact.TruncateSummary(input)

	if want := 50 + len(truncationJoiner) + 100; len(got) != want {
		t.Errorf("result length: want %d, got %d", want, len(got))
	}
	if !strings.HasSuffix(got, input[len(input)-100:]) {
		t.Errorf("result must end with the last 100 characters of the input, got %q", got)
	}
}

// ---- character (rune) counting ----

func TestTruncateSummary_MultiByteInputOf150Runes_KeptWhole(t *testing.T) {
	// 150 two-byte runes are 300 bytes: counted in bytes this would be cut.
	input := strings.Repeat("é", 150)

	got := artifact.TruncateSummary(input)

	if got != input {
		t.Errorf("150 runes must be kept whole, got %d runes", utf8.RuneCountInString(got))
	}
}

func TestTruncateSummary_MultiByteInputOver150Runes_CutByRunesAndStaysValidUTF8(t *testing.T) {
	// Mixed 1-, 2- and 3-byte runes so that any byte-based cut lands inside a sequence.
	unit := []rune{'a', 'é', '€', 'b'}
	runes := make([]rune, 200)
	for i := range runes {
		runes[i] = unit[i%len(unit)]
	}
	input := string(runes)

	got := artifact.TruncateSummary(input)

	want := string(runes[:50]) + truncationJoiner + string(runes[200-100:])
	if got != want {
		t.Errorf("multi-byte input must be cut by runes:\n  want: %q\n   got: %q", want, got)
	}
	if !utf8.ValidString(got) {
		t.Errorf("result must be valid UTF-8, got %q", got)
	}
}

// ---- error marker follows the cut ----

func TestApply_BlockedWith151CharMessage_MarkerFollowsTheCut(t *testing.T) {
	store, path, state := newMarkerStore(t)
	long := sequentialText(151)

	_, read := applyAndRead(t, store, state, blockedStep(1, domain.ErrorTOOL_UNAVAILABLE, long))

	wantCell := long[:50] + truncationJoiner + long[151-100:] + " " + errorMarkerE501
	if got := summaryCell(t, readFile(t, path), "1"); got != wantCell {
		t.Errorf("Summary cell:\n  want: %q\n   got: %q", wantCell, got)
	}
	entry := read.ExecutionLog[0]
	if entry.ErrorCode != domain.ErrorTOOL_UNAVAILABLE {
		t.Errorf("trailing-marker reader: want E501, got %q", entry.ErrorCode)
	}
	if want := long[:50] + truncationJoiner + long[151-100:]; entry.Summary != want {
		t.Errorf("parsed Summary must be the cut text without the marker:\n  want: %q\n   got: %q", want, entry.Summary)
	}
}

func TestApply_BlockedWith150CharMessage_KeptWholeBeforeMarker(t *testing.T) {
	store, path, state := newMarkerStore(t)
	whole := sequentialText(150)

	_, read := applyAndRead(t, store, state, blockedStep(1, domain.ErrorTOOL_UNAVAILABLE, whole))

	if got := summaryCell(t, readFile(t, path), "1"); got != whole+" "+errorMarkerE501 {
		t.Errorf("the marker must not count toward the 150 limit:\n  want: %q\n   got: %q", whole+" "+errorMarkerE501, got)
	}
	if got := read.ExecutionLog[0]; got.Summary != whole || got.ErrorCode != domain.ErrorTOOL_UNAVAILABLE {
		t.Errorf("parsed row: want whole summary and E501, got %q / %q", got.Summary, got.ErrorCode)
	}
}

// ---- artifacts written with the older 50 + 50 rule ----

// oldRuleSummary is a summary as the previous rule wrote it: 50 + joiner + 50.
func oldRuleSummary() string {
	src := sequentialText(300)
	return src[:50] + truncationJoiner + src[300-50:]
}

func TestParse_OldFiftyFiftySummaryWithMarker_ReadsCodeAndSummary(t *testing.T) {
	old := oldRuleSummary()

	state, err := artifact.Parse(blockedRowArtifact(old + " " + errorMarkerE501))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	entry := state.ExecutionLog[0]
	if entry.ErrorCode != domain.ErrorTOOL_UNAVAILABLE || entry.Summary != old {
		t.Errorf("want E501 / %q, got %q / %q", old, entry.ErrorCode, entry.Summary)
	}
}

func TestParse_OldFiftyFiftySummaryWithoutMarker_ReadsSummaryVerbatim(t *testing.T) {
	old := oldRuleSummary()

	state, err := artifact.Parse(blockedRowArtifact(old))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	entry := state.ExecutionLog[0]
	if entry.ErrorCode != domain.ErrorNone || entry.Summary != old {
		t.Errorf("want no code / %q, got %q / %q", old, entry.ErrorCode, entry.Summary)
	}
}

func TestRender_ParsedOldFiftyFiftySummary_IsWrittenBackUnchanged(t *testing.T) {
	old := oldRuleSummary()
	state, err := artifact.Parse(blockedRowArtifact(old + " " + errorMarkerE501))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	rendered, err := artifact.Render(state)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(string(rendered), old+" "+errorMarkerE501) {
		t.Errorf("an existing row must be re-rendered as it was written, got:\n%s", rendered)
	}
}
