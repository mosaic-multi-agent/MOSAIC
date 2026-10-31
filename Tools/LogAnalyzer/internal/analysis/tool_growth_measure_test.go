package analysis_test

// Tests for the growth of a singular call.
//
// Rules under test:
//   - Growth = next call's context - own context - own output; context is
//     input + cache read + cache write, output includes reasoning.
//   - A negative growth is left out of the figures and counted; zero is a
//     real sample.
//   - A call or next call lacking a context category, or a call lacking
//     output, is not measurable: no value, counted separately from negative.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestToolGrowth_Formula_NextContextMinusOwnContextMinusOwnOutput(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("c1", 10, 10_000, 700, tgOne("read_file")),
		tgCall("c2", 20, 13_000, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

	// 13000 - 10000 - 700; the next call's own output plays no part.
	tgAssertGrowth(t, "c1", got, domain.GrowthMeasured, "read_file", 2300)
}

func TestToolGrowth_Formula_UsesAllThreeContextCategories(t *testing.T) {
	first := ctxCall("c1", 10, 1000, 4000, 500, 100)
	first.RequestedTools = tgOne("read_file")
	next := ctxCall("c2", 20, 1200, 4300, 900, 10)
	next.RequestedTools = domain.NewRequestedTools(nil)

	run := tgAgentRun(t, first, next)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

	// (1200+4300+900) - (1000+4000+500) - 100
	tgAssertGrowth(t, "c1", got, domain.GrowthMeasured, "read_file", 800)
}

func TestToolGrowth_Output_IncludesReasoningWithoutAddingItTwice(t *testing.T) {
	first := tgCall("c1", 10, 10_000, 700, tgOne("read_file"))
	first.Usage.Reasoning = domain.Tokens(300) // part of the 700 output
	next := tgCall("c2", 20, 13_000, 50, domain.NewRequestedTools(nil))

	run := tgAgentRun(t, first, next)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

	tgAssertGrowth(t, "c1", got, domain.GrowthMeasured, "read_file", 2300)
}

func TestToolGrowth_ZeroGrowth_IsAMeasuredSample(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("c1", 10, 10_000, 500, tgOne("read_file")),
		tgCall("c2", 20, 10_500, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

	tgAssertGrowth(t, "c1", got, domain.GrowthMeasured, "read_file", 0)
}

func TestToolGrowth_NegativeGrowth_IsKeptAsNegativeAndNotAsASample(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("c1", 10, 10_000, 500, tgOne("read_file")),
		tgCall("c2", 20, 10_400, 50, domain.NewRequestedTools(nil)),
	)

	a := ctxRequireAgent(t, run, ctxAgentID)

	tgAssertGrowth(t, "c1", tgCallGrowth(t, a, "c1"), domain.GrowthNegative, "read_file", -100)
}

func TestToolGrowth_NegativeGrowth_IsCountedAndLeftOutOfTheToolFigures(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("c1", 10, 10_000, 500, tgOne("read_file")),
		tgCall("c2", 20, 10_400, 50, tgOne("read_file")),            // c1 growth -100
		tgCall("c3", 30, 11_000, 50, domain.NewRequestedTools(nil)), // c2 growth 550
	)

	tool := requireTool(t, run.Tools.Run, "read_file")

	if tool.Growth.Samples != 1 || tool.Growth.Negative != 1 || tool.Growth.NotMeasurable != 0 {
		t.Fatalf("growth = %+v, want 1 sample, 1 negative, 0 not measurable", tool.Growth)
	}
	ctxAssertTokens(t, "average", tool.Growth.Average, 550)
	ctxAssertTokens(t, "min", tool.Growth.Min, 550)
	ctxAssertTokens(t, "max", tool.Growth.Max, 550)
}

func TestToolGrowth_MissingContextCategory_IsNotMeasurable(t *testing.T) {
	cases := map[string]func(u *domain.InvocationUsage){
		"input absent":       func(u *domain.InvocationUsage) { u.Usage.Input = domain.AbsentTokens() },
		"cache read absent":  func(u *domain.InvocationUsage) { u.Usage.CacheRead = domain.AbsentTokens() },
		"cache write absent": func(u *domain.InvocationUsage) { u.Usage.CacheCreation = domain.AbsentTokens() },
		"output absent":      func(u *domain.InvocationUsage) { u.Usage.Output = domain.AbsentTokens() },
	}
	for name, strip := range cases {
		t.Run("own call, "+name, func(t *testing.T) {
			first := tgCall("c1", 10, 10_000, 500, tgOne("read_file"))
			strip(&first)

			run := tgAgentRun(t, first, tgCall("c2", 20, 12_000, 50, domain.NewRequestedTools(nil)))

			got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

			tgAssertGrowth(t, "c1", got, domain.GrowthNotMeasurable, "read_file", 0)
		})
	}
	for name, strip := range cases {
		if name == "output absent" {
			continue // the next call's output is not used by the formula
		}
		t.Run("next call, "+name, func(t *testing.T) {
			next := tgCall("c2", 20, 12_000, 50, domain.NewRequestedTools(nil))
			strip(&next)

			run := tgAgentRun(t, tgCall("c1", 10, 10_000, 500, tgOne("read_file")), next)

			got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

			tgAssertGrowth(t, "c1", got, domain.GrowthNotMeasurable, "read_file", 0)
		})
	}
}

func TestToolGrowth_NextCallWithoutOutput_StillMeasures(t *testing.T) {
	next := tgCall("c2", 20, 12_000, 50, domain.NewRequestedTools(nil))
	next.Usage.Output = domain.AbsentTokens()

	run := tgAgentRun(t, tgCall("c1", 10, 10_000, 500, tgOne("read_file")), next)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

	tgAssertGrowth(t, "c1", got, domain.GrowthMeasured, "read_file", 1500)
}

func TestToolGrowth_NotMeasurable_IsCountedSeparatelyFromNegative(t *testing.T) {
	partial := tgCall("c1", 10, 10_000, 500, tgOne("read_file"))
	partial.Usage.CacheRead = domain.AbsentTokens()

	run := tgAgentRun(t,
		partial,
		tgCall("c2", 20, 12_000, 500, tgOne("read_file")), // measured 500
		tgCall("c3", 30, 13_000, 50, domain.NewRequestedTools(nil)),
	)

	tool := requireTool(t, run.Tools.Run, "read_file")

	if tool.Growth.Samples != 1 || tool.Growth.Negative != 0 || tool.Growth.NotMeasurable != 1 {
		t.Errorf("growth = %+v, want 1 sample, 0 negative, 1 not measurable", tool.Growth)
	}
}
