package screens_test

// Helpers of the collapsed-axis Timeline tests: a hand-built run whose geometry is
// exact, and the long-break fixture analysed through the real service.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/pricing"
)

// longBreakDir holds a run of 3 hours of work with a break of 3 days and 2 hours
// between its two sessions.
const longBreakDir = "../../../testdata/longbreak/logs"

// longBreakReport analyses the long-break fixture through the real service.
func longBreakReport(t *testing.T) domain.RunReport {
	t.Helper()
	root := t.TempDir()
	cfg := filepath.Join(root, filepath.FromSlash(pricing.ConfigRelPath))
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(samplePricingYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := app.New(app.Deps{
		Source: logscan.NewScanner(), Reader: logread.NewReader(), Pricing: pricing.NewStore("", root),
		Digests: digest.NewStore(), Clock: fixedClock{}, Interaction: quietInteraction{},
		WorkDir: root, ToolVersion: "test",
	})
	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: longBreakDir})
	if err != nil {
		t.Fatalf("analyse the long-break fixture: %v", err)
	}
	if len(res.Report.Runs) != 1 {
		t.Fatalf("long-break report has %d runs, want 1", len(res.Report.Runs))
	}
	return res.Report.Runs[0]
}

// collapseWork is the working time on each side of every collapsed stretch of the
// hand-built runs: 3500 seconds, which on a 70 cell work area is 35 cells a side.
const collapseWork = 3500

// collapseGap is 3 days and 2 hours in seconds.
const collapseGap = 266_400

// brokenRun has one invocation on each side of a 3d 2h break; both fill their side
// of the axis. The break is between sessions and collapsed.
func brokenRun() domain.RunReport {
	run := overviewRun()
	first := tlWithStatus(tlClosed("w1#1", "coder", tlAt(0), tlAt(collapseWork)), "SUCCESS")
	second := tlWithStatus(tlClosed("w2#2", "tester", tlAt(collapseWork+collapseGap), tlAt(2*collapseWork+collapseGap)), "SUCCESS")
	brk := tlSpan(collapseWork, collapseWork+collapseGap)
	run.Timeline = domain.Timeline{
		Entries: []domain.TimelineEntry{first, second},
		Axis:    tlSpan(0, 2*collapseWork+collapseGap),
		HasAxis: true,
	}
	run.WallTime = domain.WallTimeSplit{
		Span: run.Timeline.Axis, HasSpan: true, BetweenSessions: brk.End.Sub(brk.Start),
		BetweenSessionIntervals: []domain.TimeInterval{brk}, Collapsible: []domain.TimeInterval{brk},
	}
	return run
}

// waitingRun is one invocation that waits on the user for most of its life; the
// wait is collapsed and the invocation's bar continues on both sides of it.
func waitingRun() domain.RunReport {
	const wait = 20_000
	run := overviewRun()
	long := tlWithStatus(tlClosed("long#1", "coder", tlAt(0), tlAt(2*collapseWork+wait)), "SUCCESS")
	stretch := tlSpan(collapseWork, collapseWork+wait)
	run.Timeline = domain.Timeline{Entries: []domain.TimelineEntry{long}, Axis: tlSpan(0, 2*collapseWork+wait), HasAxis: true}
	run.WallTime = domain.WallTimeSplit{
		Span: run.Timeline.Axis, HasSpan: true, Waiting: stretch.End.Sub(stretch.Start),
		WaitIntervals: []domain.TimeInterval{stretch}, Collapsible: []domain.TimeInterval{stretch},
		ActorWaits: []domain.ActorWaitIntervals{{Actor: long.Invocation.Actor, Intervals: []domain.TimeInterval{stretch}}},
	}
	return run
}

// brokenWidth gives a plot area of 80 cells: the 12 cell orchestrator label, a blank and the plot.
const brokenWidth = len("orchestrator") + 1 + 80

// plotOfLane is the plot area of the lane of an invocation.
func plotOfLane(t *testing.T, view, id string) string {
	t.Helper()
	return plotPart(laneLine(t, view, id, timelineTypes2[id]))
}

// timelineTypes2 are the types of the invocations of the collapse fixtures.
var timelineTypes2 = map[string]string{"w1#1": "coder", "w2#2": "tester", "long#1": "coder", "worker#1": "", "tester#2": ""}

// secs is a duration in whole seconds.
func secs(n int) time.Duration { return time.Duration(n) * time.Second }

func containsInt(list []int, n int) bool {
	for _, v := range list {
		if v == n {
			return true
		}
	}
	return false
}

func repStr(s string, n int) string { return strings.Repeat(s, n) }
