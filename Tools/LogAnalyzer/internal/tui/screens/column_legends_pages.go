package screens

import "mosaic-common/tui/kit/chrome"

// Column descriptions of the overview, the timeline and the page screens.

// overviewLegend is the legend of the tables of the Overview.
func overviewLegend() []chrome.HelpGroup {
	return []chrome.HelpGroup{
		legendGroup("cost summary", []columnDoc{
			{"figure", "The part of the run's cost that the row reports."},
			{"cost", "What the figure cost in the display currency."},
			{"share", "The figure's share of the run's total cost."},
			{"note", "What the figure covers or why it is incomplete."},
		}),
		legendGroup("tokens", []columnDoc{
			{"category", "The kind of tokens that the row sums: input, cache read, cache write or output."},
			{"tokens", "How many billable tokens of the category the run used."},
			{"cost", "What the tokens of the category cost."},
			{"share", "The category's share of the run's total cost."},
		}),
		legendGroup("models", []columnDoc{
			{"model", "The model that served the API calls of the row."},
			{"API calls", "How many API calls the model answered."},
			{"cost", "What the model's API calls cost."},
			{"tokens", "Billable tokens the model's API calls used."},
			{"share", "The model's share of the run's total cost."},
		}),
		legendGroup("outcomes", []columnDoc{
			{"outcome", "The status code that the row groups dispatches by."},
			{"dispatches", "How many dispatches returned the status."},
			{"share", "The status's share of all dispatches of the run."},
		}),
		legendGroup("wall time", []columnDoc{
			{"category", "What the time was spent on, such as model calls, tools or waiting."},
			{"time", "How much wall-clock time the category took."},
			{"share", "The category's share of the run's wall-clock time."},
		}),
		legendGroup("waits", []columnDoc{
			{"tools counted as waits", "A tool whose duration counts as waiting for the user, not as work."},
			{"harness", "The harness whose log named the tool."},
			{"calls", "How many calls of the tool the run made."},
		}),
	}
}

// timelineLegend is the legend of the two lists of the Timeline.
func timelineLegend() []chrome.HelpGroup {
	return []chrome.HelpGroup{
		legendGroup("invocations", []columnDoc{
			{"instance", "The dispatched agent instance, named by its agent type and number."},
			{"start", "When the instance started."},
			{"end", "When the instance ended."},
			{"duration", "Wall-clock time from the start to the end of the instance."},
			{"status", "The status code the instance returned to its caller."},
			{"overlap", "How much of the instance's time ran at the same time as other instances."},
		}),
		legendGroup("compactions", []columnDoc{
			{"time", "When the compaction happened."},
			{"actor", "The agent whose context was compacted."},
			{"trigger", "What caused the compaction, as the harness logged it."},
			{"before", "The context size before the compaction."},
			{"after", "The context size after the compaction."},
			{"shape", "How the compaction was logged: as a pair of events or as one unpaired half."},
		}),
	}
}

// acrossRunsLegend is the legend of the two tables of the across-runs view.
func acrossRunsLegend() []chrome.HelpGroup {
	return []chrome.HelpGroup{
		legendGroup("runs", []columnDoc{
			{"run", "The name of the run."},
			{"total", "What the whole run cost in the display currency."},
			{"share", "The run's share of the cost of all listed runs."},
			{"subagents", "How many subagent instances the run dispatched."},
			{"flags", "Findings about the run, such as missing usage or unpriced models."},
		}),
		legendGroup("agent types", []columnDoc{
			{"type", "The agent type that the row sums over all runs."},
			{"invocations", "How many instances of the type were dispatched over all runs."},
			{"runs", "In how many runs the type was dispatched."},
			{"total", "The summed cost of the type over all runs."},
			{"share", "The type's share of the cost of all listed runs."},
			{"average per run", "The type's cost per run in which it appeared."},
			{"invocations per run", "How many instances of the type a run dispatched on average."},
			{"tokens per run", "How many billable tokens the type used per run on average."},
			{"flags", "Findings about instances of the type, such as missing usage."},
		}),
	}
}

// compareLegend is the legend of the comparison of two runs.
func compareLegend() []chrome.HelpGroup {
	return []chrome.HelpGroup{legendGroup("", []columnDoc{
		{"type", "The agent type that the row compares between run A and run B."},
		{"note", "Says when the type was dispatched in one of the runs only."},
		{"invocations A", "How many instances of the type run A dispatched."},
		{"invocations B", "How many instances of the type run B dispatched."},
		{"invocations change", "Invocations of run B minus invocations of run A."},
		{"tokens A", "Billable tokens the type used in run A."},
		{"tokens B", "Billable tokens the type used in run B."},
		{"tokens change", "Tokens of run B minus tokens of run A."},
		{"cost A", "What the type cost in run A."},
		{"cost B", "What the type cost in run B."},
		{"cost change", "Cost of run B minus cost of run A."},
		{"cost change percent", "The cost change as a percentage of the cost in run A."},
		{"peak context A", "The average peak context of the type's instances in run A."},
		{"peak context B", "The average peak context of the type's instances in run B."},
		{"peak context change", "Average peak context of run B minus that of run A."},
		{"peak context change percent", "The peak context change as a percentage of run A's."},
	})}
}

// runsLegend is the legend of the runs list, including its mark column.
func runsLegend() []chrome.HelpGroup {
	return []chrome.HelpGroup{legendGroup("", []columnDoc{
		{"compare", "A or B for a run marked for a comparison, empty for an unmarked run."},
		{"run", "The name of the run."},
		{"cost", "What the whole run cost in the display currency."},
		{"invocations", "How many agent instances the run dispatched."},
		{"duration", "Wall-clock time from the start to the end of the run."},
		{"markers", "Findings about the run, such as missing usage or unpriced models."},
	})}
}

// pricingLegend is the legend of the pricing screen.
func pricingLegend() []chrome.HelpGroup {
	return []chrome.HelpGroup{legendGroup("", []columnDoc{
		{"model", "The model name as the logs record it."},
		{"status", "Whether the model has a price; an unpriced model has no cost."},
		{"source", "The price file level the price was loaded from."},
	})}
}
