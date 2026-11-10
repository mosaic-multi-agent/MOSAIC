package screens_test

// Column titles are full names: no abbreviation, no note-dependent title, the
// figure of API requests is titled "API calls", and the Context table puts one
// label over its compaction columns.

import (
	"strings"
	"testing"
)

// fullTitles are the titles each table must show, by view name. Only titles that
// are renamed (or sit beside renamed ones) are listed; the rest is covered by the
// abbreviation rule below.
var fullTitles = map[string][]string{
	"dispatches":         {"instance", "status", "cost", "peak context", "cache hit", "cache write", "write cost", "net cache", "unread cache writes"},
	"by type drill-down": {"instance", "status", "cost", "peak context", "cache hit", "cache write", "write cost"},
	"by type":            {"type", "invocations", "total", "average", "average peak context", "maximum peak context", "cache hit", "repeat spend"},
	"by status":          {"status", "dispatches", "dispatch share", "cost", "cost share", "flags"},
	"tools":              {"tool", "calls", "errors", "average duration", "minimum duration", "maximum duration", "unpaired"},
	"context": {"instance", "model", "rewrites", "API calls", "first context", "context at first write", "peak context",
		"final context", "logged", "harness", "detected only", "unapplied", "cost"},
	"overview models":            {"model", "API calls", "cost", "tokens"},
	"across runs, agent types":   {"type", "invocations", "runs", "total", "average per run", "invocations per run", "tokens per run"},
	"runs list":                  {"compare", "run", "cost", "invocations", "duration", "markers"},
	"runs list with marked runs": {"compare", "run", "cost", "invocations", "duration", "markers"},
	"compare":                    {"type", "invocations A", "invocations B", "tokens A", "tokens B", "cost A", "cost B"},
	"pricing":                    {"model", "status", "source"},
}

// abbreviated are titles and title words that are shortened names, or that point
// at a note; none may be on screen.
var abbreviatedWords = []string{"avg", "min", "max", "cmp", "ctx", "inv", "chg", "tok", "disp.", "det.", "avg/run", "inv/run", "tokens/run"}

func TestTableTitles_EveryViewShowsFullNames(t *testing.T) {
	for _, v := range headerViews() {
		want, ok := fullTitles[v.name]
		if !ok {
			continue
		}
		t.Run(v.name, func(t *testing.T) {
			block := readHeader(t, v.view(t, headerW, headerH), v.anchor)

			for _, title := range want {
				if _, shown := block.col(title); !shown {
					t.Errorf("no column is titled %q; the titles are %q", title, block.titles())
				}
			}
			if block.hidden {
				t.Errorf("a column was dropped at %d columns, so its title could not be checked: %q", headerW, block.titles())
			}
		})
	}
}

func TestTableTitles_NoViewShowsAnAbbreviatedOrNoteDependentTitle(t *testing.T) {
	for _, v := range headerViews() {
		t.Run(v.name, func(t *testing.T) {
			block := readHeader(t, v.view(t, headerW, headerH), v.anchor)

			for _, c := range block.cols {
				for _, word := range c.words {
					for _, bad := range abbreviatedWords {
						if word == bad {
							t.Errorf("title %q uses the shortened word %q", c.title, bad)
						}
					}
					if strings.HasSuffix(word, "*") || strings.HasSuffix(word, ".") || strings.Contains(word, "/") {
						t.Errorf("title %q has the word %q, which is shortened or points at a note", c.title, word)
					}
				}
			}
		})
	}
}

func TestTableTitles_ContextTitlesNoneOfTheOldNamesRemain(t *testing.T) {
	block := readHeader(t, contextHeaderView(t), "instance")

	for _, old := range []string{"calls", "first", "peak", "final", "first write*", "det. only", "compaction cost"} {
		if _, shown := block.col(old); shown {
			t.Errorf("the Context table still has a column titled %q", old)
		}
	}
}

func TestTableTitles_ToolsDurationsNameTheirMeasure(t *testing.T) {
	block := readHeader(t, viewNamed(t, "tools"), "tool")

	for _, old := range []string{"avg", "min", "max"} {
		if _, shown := block.col(old); shown {
			t.Errorf("the Tools table still has a column titled %q", old)
		}
	}
}

func TestTableTitles_RequestFigureIsTitledAPICallsInContextAndOverviewModels(t *testing.T) {
	for _, name := range []string{"context", "overview models"} {
		t.Run(name, func(t *testing.T) {
			block := readHeader(t, viewNamed(t, name), viewAnchor(t, name))

			if _, shown := block.col("API calls"); !shown {
				t.Errorf("no column is titled \"API calls\"; the titles are %q", block.titles())
			}
			for _, old := range []string{"calls", "requests"} {
				if _, shown := block.col(old); shown {
					t.Errorf("a column is still titled %q", old)
				}
			}
		})
	}
}

func TestTableTitles_ContextPutsOneLabelOverTheCompactionColumns(t *testing.T) {
	block := readHeader(t, contextHeaderView(t), "instance")

	if !strings.Contains(block.group, "compactions") {
		t.Fatalf("the Context header has no group line naming compactions: %q", block.group)
	}
	first, okFirst := block.col("logged")
	last, okLast := block.col("cost")
	if !okFirst || !okLast {
		t.Fatalf("the compaction columns logged and cost are missing: %q", block.titles())
	}
	startCol, endCol := ruleExtent([]rune(block.group)[gutterWidth:])
	if startCol != first.start || endCol != last.end {
		t.Errorf("the label spans display columns [%d, %d), want [%d, %d) from logged to cost", startCol, endCol, first.start, last.end)
	}
	if block.groupLine != 0 {
		t.Errorf("the group line must be the top header line, it is line %d of %d", block.groupLine, block.headerLines)
	}
}

// ruleExtent is the display columns [start, end) from the first to the last rule
// cell of a group line.
func ruleExtent(cells []rune) (start, end int) {
	start, end = -1, -1
	for i, c := range cells {
		if c == '─' {
			if start < 0 {
				start = i
			}
			end = i + 1
		}
	}
	return start, end
}
