package artifact_test

// Tests for the shared wildcard matcher and for the production written-output
// detector's use of it. The matcher lists, for a slash-separated pattern
// relative to a root folder, every existing path that matches it, in
// per-segment lexical order. Only '*' is special: it stands for any text
// within one path segment and never crosses '/'. '?', '[', ']' and '\' match
// only themselves, and there is no escape character.
//
// Matcher tests run over an in-memory file system, which can hold names that
// the platform file system does not allow.

import (
	"context"
	"reflect"
	"testing"
	"testing/fstest"

	"mosaic-run/internal/artifact"
)

// memFS builds an in-memory file system holding the given file paths.
func memFS(files ...string) fstest.MapFS {
	m := fstest.MapFS{}
	for _, f := range files {
		m[f] = &fstest.MapFile{Data: []byte("x")}
	}
	return m
}

// requireExpansion asserts that pattern over fsys lists exactly want, in order.
func requireExpansion(t *testing.T, files []string, pattern string, want []string) {
	t.Helper()
	got := artifact.ExpandWildcard(memFS(files...), pattern)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandWildcard(%q) over %q: want %q, got %q", pattern, files, want, got)
	}
}

func TestExpandWildcard_StarWithinOneSegment_ListsMatchingPaths(t *testing.T) {
	files := []string{"Stage-A/Plan.md", "Stage-b-2/Plan.md", "Other/Plan.md", "Stage-A/Notes.md"}

	requireExpansion(t, files, "Stage-*/Plan.md",
		[]string{"Stage-A/Plan.md", "Stage-b-2/Plan.md"})
}

func TestExpandWildcard_StarFormsOtherThanStagePrefix_ListsMatchingPaths(t *testing.T) {
	files := []string{"alpha-Research.md", "beta-Research.md", "Research.md", "alpha-Notes.md"}

	requireExpansion(t, files, "*-Research.md",
		[]string{"alpha-Research.md", "beta-Research.md"})
}

func TestExpandWildcard_StarMatchesEmptyText(t *testing.T) {
	files := []string{"Stage-/Plan.md", "Stage-A/Plan.md"}

	requireExpansion(t, files, "Stage-*/Plan.md", []string{"Stage-/Plan.md", "Stage-A/Plan.md"})
}

func TestExpandWildcard_SeveralStarsInOneSegment_AllListed(t *testing.T) {
	files := []string{"abcde.md", "ace.md", "abde.md", "ace.txt"}

	requireExpansion(t, files, "a*c*e.md", []string{"abcde.md", "ace.md"})
}

func TestExpandWildcard_StarNeverCrossesSlash(t *testing.T) {
	t.Run("last segment", func(t *testing.T) {
		requireExpansion(t, []string{"a.md", "sub/b.md"}, "*.md", []string{"a.md"})
	})
	t.Run("middle segment", func(t *testing.T) {
		requireExpansion(t, []string{"Stage-A/Plan.md", "Stage-A/deep/Plan.md"}, "Stage-*/Plan.md",
			[]string{"Stage-A/Plan.md"})
	})
	t.Run("star alone is one directory level", func(t *testing.T) {
		requireExpansion(t, []string{"x/y/z.md"}, "*/z.md", nil)
	})
}

func TestExpandWildcard_SeveralMatches_ReturnedInLexicalOrder(t *testing.T) {
	// Inserted out of order on purpose; bytewise order puts upper case first.
	files := []string{"Stage-C/Plan.md", "Stage-b-2/Plan.md", "Stage-A/Plan.md"}

	requireExpansion(t, files, "Stage-*/Plan.md",
		[]string{"Stage-A/Plan.md", "Stage-C/Plan.md", "Stage-b-2/Plan.md"})
}

func TestExpandWildcard_OrderIsPerSegment(t *testing.T) {
	// "a" sorts before "a-b" as a segment, although "a-b/x" sorts before
	// "a/x" as a whole string ('-' is 0x2D, '/' is 0x2F).
	files := []string{"a-b/x", "a/x"}

	requireExpansion(t, files, "*/x", []string{"a/x", "a-b/x"})
}

func TestExpandWildcard_NoMatch_ReturnsEmptyList(t *testing.T) {
	t.Run("nothing matches", func(t *testing.T) {
		requireExpansion(t, []string{"Stage-A/Plan.md"}, "Absent-*/Plan.md", nil)
	})
	t.Run("directory missing", func(t *testing.T) {
		requireExpansion(t, []string{"Plan.md"}, "Missing/Stage-*/Plan.md", nil)
	})
	t.Run("empty file system", func(t *testing.T) {
		requireExpansion(t, nil, "*.md", nil)
	})
}

func TestExpandWildcard_PatternWithoutStar_ReturnedAsIsWhetherOrNotItExists(t *testing.T) {
	t.Run("existing path", func(t *testing.T) {
		requireExpansion(t, []string{"Plan.md"}, "Plan.md", []string{"Plan.md"})
	})
	t.Run("missing path", func(t *testing.T) {
		requireExpansion(t, []string{"Plan.md"}, "Stage-A/Plan.md", []string{"Stage-A/Plan.md"})
	})
	t.Run("question mark and bracket are not wildcards", func(t *testing.T) {
		requireExpansion(t, []string{"notes-1.md"}, "notes-[1].md", []string{"notes-[1].md"})
		requireExpansion(t, []string{"notes-1.md"}, "notes-?.md", []string{"notes-?.md"})
	})
}

func TestExpandWildcard_QuestionMarkMatchesOnlyItself(t *testing.T) {
	files := []string{"what?-1.md", "whatx-2.md"}

	requireExpansion(t, files, "what?-*.md", []string{"what?-1.md"})
}

func TestExpandWildcard_OpeningBracketMatchesOnlyItself(t *testing.T) {
	// The old path.Match reading would treat [ab] as a character class and
	// match "a-x.md" and "b-x.md" but not the literal name.
	files := []string{"[ab]-x.md", "a-x.md", "b-x.md"}

	requireExpansion(t, files, "[ab]*.md", []string{"[ab]-x.md"})
}

func TestExpandWildcard_ClosingBracketMatchesOnlyItself(t *testing.T) {
	files := []string{"a]-1.md", "a-1.md"}

	requireExpansion(t, files, "a]*.md", []string{"a]-1.md"})
}

func TestExpandWildcard_BackslashMatchesOnlyItself_NoEscapeCharacter(t *testing.T) {
	// The old path.Match reading would treat \* as a literal star and match
	// only the file named "back*.md".
	files := []string{`back\one.md`, "back*.md", "backone.md"}

	requireExpansion(t, files, `back\*.md`, []string{`back\one.md`})
}

func TestExpandWildcard_BackslashIsNotASeparator(t *testing.T) {
	files := []string{`Stage-A\Plan.md`, "Stage-A/Plan.md"}

	requireExpansion(t, files, `Stage-*\Plan.md`, []string{`Stage-A\Plan.md`})
}

func TestExpandWildcard_NonLastSegmentsMustBeDirectories(t *testing.T) {
	// "Stage-A" is a file here, so nothing lies beneath it.
	requireExpansion(t, []string{"Stage-A", "Stage-B/Plan.md"}, "Stage-*/Plan.md",
		[]string{"Stage-B/Plan.md"})
}

func TestExpandWildcard_LastSegmentMayBeADirectory(t *testing.T) {
	files := []string{"Stage-A/Plan.md", "Stage-B/Plan.md", "Stage-notes.md"}

	requireExpansion(t, files, "Stage-*", []string{"Stage-A", "Stage-B", "Stage-notes.md"})
}

func TestExpandWildcard_LiteralLeadingSegmentsCopiedAsWritten(t *testing.T) {
	run := "Orchestration-20260928T103906Z-d408"
	files := []string{run + "/Stage-A/Plan.md", run + "/Stage-b-2/Plan.md"}

	requireExpansion(t, files, run+"/Stage-*/Plan.md",
		[]string{run + "/Stage-A/Plan.md", run + "/Stage-b-2/Plan.md"})
}

func TestExpandWildcard_PatternOutsideTheRoot_YieldsNoMatch(t *testing.T) {
	files := []string{"a.md", "sub/b.md"}

	for _, pattern := range []string{"../*.md", "/*.md", "sub/../*.md", "sub/*/"} {
		requireExpansion(t, files, pattern, nil)
	}
}

// ---- production written-output detector ----

func TestOutputWriteDetector_Wildcard_ReportsExactlyTheMatchingFilesWritten(t *testing.T) {
	root := t.TempDir()
	d := artifact.NewOutputWriteDetector()
	ctx := context.Background()
	detWrite(t, root, detRunDir+"/Stage-A/Plan.md", "old A\n")

	base := d.Baseline(ctx, detQuery(root, detRunDir+"/Stage-*/Plan.md"))
	detWrite(t, root, detRunDir+"/Stage-A/Plan.md", "new A\n")
	detWrite(t, root, detRunDir+"/Stage-b-2/Plan.md", "created\n")
	detWrite(t, root, detRunDir+"/Stage-C/Notes.md", "not matched\n")
	detWrite(t, root, detRunDir+"/Other/Plan.md", "not matched\n")
	got := d.Written(ctx, base)

	want := []string{detRunDir + "/Stage-A/Plan.md", detRunDir + "/Stage-b-2/Plan.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wildcard output: want exactly %v, got %v", want, got)
	}
}

func TestOutputWriteDetector_Wildcard_FormOtherThanStagePrefix_ReportsMatches(t *testing.T) {
	root := t.TempDir()
	d := artifact.NewOutputWriteDetector()
	ctx := context.Background()

	base := d.Baseline(ctx, detQuery(root, detRunDir+"/*-Research.md"))
	detWrite(t, root, detRunDir+"/alpha-Research.md", "a\n")
	detWrite(t, root, detRunDir+"/Research.md", "not matched\n")
	detWrite(t, root, detRunDir+"/sub/beta-Research.md", "not matched, other level\n")
	got := d.Written(ctx, base)

	want := []string{detRunDir + "/alpha-Research.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wildcard output: want exactly %v, got %v", want, got)
	}
}

func TestOutputWriteDetector_QuestionMarkInDeclaredPath_IsLiteral(t *testing.T) {
	// "draft-a.md" would match a "draft-?.md" wildcard; it must not be
	// reported for a declared path that merely contains a question mark.
	root := t.TempDir()
	d := artifact.NewOutputWriteDetector()
	ctx := context.Background()
	declared := detRunDir + "/draft-?.md"

	base := d.Baseline(ctx, detQuery(root, declared))
	detWrite(t, root, detRunDir+"/draft-a.md", "created\n")
	got := d.Written(ctx, base)

	if len(got) != 0 {
		t.Errorf("declared path with '?': want nothing reported (no file of that literal name), got %v", got)
	}
}

func TestOutputWriteDetector_OpeningBracketInDeclaredPath_IsLiteral(t *testing.T) {
	// "notes-1.md" would match the character class in "notes-[1].md".
	root := t.TempDir()
	d := artifact.NewOutputWriteDetector()
	ctx := context.Background()
	declared := detRunDir + "/notes-[1].md"

	base := d.Baseline(ctx, detQuery(root, declared))
	detWrite(t, root, detRunDir+"/notes-1.md", "created\n")
	got := d.Written(ctx, base)

	if len(got) != 0 {
		t.Errorf("declared path with '[': want nothing reported (no file of that literal name), got %v", got)
	}
}

func TestOutputWriteDetector_FileNamedWithBracket_ReportedByItsLiteralName(t *testing.T) {
	root := t.TempDir()
	d := artifact.NewOutputWriteDetector()
	ctx := context.Background()
	declared := detRunDir + "/notes-[1].md"

	base := d.Baseline(ctx, detQuery(root, declared))
	detWrite(t, root, declared, "created\n")
	got := d.Written(ctx, base)

	if !reflect.DeepEqual(got, []string{declared}) {
		t.Errorf("literal bracket name: want [%s], got %v", declared, got)
	}
}

func TestOutputWriteDetector_StarWithBracketInPattern_OnlyBracketLiteralMatches(t *testing.T) {
	root := t.TempDir()
	d := artifact.NewOutputWriteDetector()
	ctx := context.Background()

	base := d.Baseline(ctx, detQuery(root, detRunDir+"/part-[1]-*.md"))
	detWrite(t, root, detRunDir+"/part-[1]-a.md", "literal\n")
	detWrite(t, root, detRunDir+"/part-1-b.md", "class match only\n")
	got := d.Written(ctx, base)

	want := []string{detRunDir + "/part-[1]-a.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pattern with '[' and '*': want exactly %v, got %v", want, got)
	}
}
