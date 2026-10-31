package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"

	"mosaic-log-analyzer/internal/domain"
)

// The column legend of the ? overlay: every view with a table lists the columns it
// renders, one sentence each, in "Columns" groups between the screen's own keys and
// the navigation keys. A screen registers its descriptions by handing its legend
// groups to the overlay (frameSpec.legend for per-run views, withLegend for pages).

// columnDoc describes one column: its title exactly as the table shows it.
type columnDoc struct{ title, desc string }

// legendGroup is the overlay group of one table; name says which table when the
// view has several, and is empty for a view with a single one.
func legendGroup(name string, docs []columnDoc) chrome.HelpGroup {
	title := "Columns"
	if name != "" {
		title += ": " + name
	}
	entries := make([]tuicommon.HelpEntry, len(docs))
	for i, d := range docs {
		entries[i] = tuicommon.HelpEntry{Key: d.title, Desc: d.desc}
	}
	return chrome.HelpGroup{Title: title, Entries: entries}
}

// withLegend inserts the legend groups right before the Navigation group of a page's
// overlay groups.
func withLegend(groups []chrome.HelpGroup, legend ...chrome.HelpGroup) []chrome.HelpGroup {
	for i, g := range groups {
		if g.Title == "Navigation" {
			out := append([]chrome.HelpGroup(nil), groups[:i]...)
			out = append(out, legend...)
			return append(out, groups[i:]...)
		}
	}
	return append(groups, legend...)
}

// dispatchDocs describe the columns of a table of dispatches.
func dispatchDocs() []columnDoc {
	return []columnDoc{
		{"instance", "The dispatched agent instance, named by its agent type and number."},
		{"status", "The status code the instance returned to its caller."},
		{"cost", "What the instance's API calls cost in the display currency."},
		{"share", "The instance's share of the run's total cost."},
		{"tokens", "Billable tokens of the instance: input, cache read, cache write and output."},
		{"duration", "Wall-clock time from the start to the end of the instance."},
		{"peak context", "The largest context any API call of the instance sent."},
		{"cache hit", "The share of the instance's input served from the prompt cache."},
		{"rewrites", "API calls that wrote their context again instead of reading it from the cache."},
		{"cache write", "Tokens the instance wrote to the prompt cache."},
		{"write cost", "What the instance's cache writes cost."},
		{"net cache", "Cache savings minus the cost of cache writes; negative when writes were not paid back."},
		{"unread cache writes", "Tokens written to the cache that no later call of the instance read."},
		{"effort", "The reasoning effort levels the instance's requests used."},
		{"flags", "Findings about the instance, such as missing usage or an unfinished run."},
	}
}

// typeDocs describe the columns of the by-type table.
func typeDocs() []columnDoc {
	return []columnDoc{
		{"type", "The agent type that the row sums over all its instances."},
		{"invocations", "How many instances of the type were dispatched."},
		{"total", "The summed cost of all instances of the type."},
		{"average", "The cost of an average instance of the type."},
		{"minimum", "The cost of the cheapest instance of the type."},
		{"maximum", "The cost of the most expensive instance of the type."},
		{"share", "The type's share of the run's total cost."},
		{"flags", "Findings about instances of the type, such as missing usage."},
		{"average peak context", "The mean of the largest context each instance of the type sent."},
		{"maximum peak context", "The largest context any instance of the type sent."},
		{"cache hit", "The share of the type's input served from the prompt cache."},
		{"rewrites", "API calls of the type that wrote their context again instead of reading the cache."},
		{"cache write", "Tokens instances of the type wrote to the prompt cache."},
		{"write cost", "What the type's cache writes cost."},
		{"repeats", "Dispatches of the type that repeat an earlier dispatch, planned ones included."},
		{"repeat spend", "What the repeated dispatches of the type cost, planned repeats included."},
		{"likely rework", "Repeats that look like redone work; a heuristic, not a proven fact."},
		{"rework cost", "What the likely rework of the type cost."},
	}
}

// statusDocs describe the columns of the by-status table.
func statusDocs() []columnDoc {
	return []columnDoc{
		{"status", "The status code that the row groups dispatches by."},
		{"dispatches", "How many dispatches returned the status."},
		{"dispatch share", "The status's share of all dispatches of the run."},
		{"cost", "What the dispatches with the status cost together."},
		{"cost share", "The status's share of the run's total cost."},
		{"flags", "Findings about the dispatches with the status, such as missing usage."},
	}
}

// toolDocs describe the columns of the tools table.
func toolDocs() []columnDoc {
	return []columnDoc{
		{"tool", "The tool name as the harness logged it."},
		{"calls", "How many times the tool was called."},
		{"errors", "How many calls of the tool ended in an error."},
		{"average duration", "The mean time a call of the tool took."},
		{"minimum duration", "The time the quickest call of the tool took."},
		{"maximum duration", "The time the slowest call of the tool took."},
		{"unpaired", "Calls whose start or end was not logged, so they have no duration."},
		{"share", "The tool's share of all tool calls of the run."},
	}
}

// growthDocs describe the context growth columns of the tools table.
func growthDocs() []columnDoc {
	return []columnDoc{
		{"average growth", "The mean tokens a call of the tool added to the next call's context."},
		{"minimum growth", "The smallest number of tokens a call of the tool added to the context."},
		{"maximum growth", "The largest number of tokens a call of the tool added to the context."},
		{"growth samples", "The calls that requested exactly one tool, the only ones growth is measured on."},
		{"negative growth", "Samples where the context shrank, for example after a compaction; left out of the figures."},
		{"not measurable", "Calls whose growth could not be measured; left out of the figures."},
	}
}

// contextDocs describe the columns of the Context table, including the heuristic
// behind the first write figure, the logged and detected compactions and the
// definition of final context.
func contextDocs() []columnDoc {
	return []columnDoc{
		{"instance", "The actor of the row: the orchestrator first, then each dispatched instance."},
		{"model", "The models the actor's main calls used, in order of first use."},
		{"rewrites", "API calls that wrote their context again instead of reading it from the cache."},
		{"API calls", "How many API calls the actor made."},
		{"first context", "The context of the actor's first call."},
		{"context at first write", "A heuristic: the context at the first write-class tool call; not computed for the orchestrator."},
		{"peak context", "The largest context any call of the actor sent."},
		{"final context", "The context at the end of the actor's run: " + domain.FinalContextDefinition + "."},
		{"logged", "Compactions the harness recorded in its own log."},
		{"harness", "Compactions the harness reported through OpenTelemetry."},
		{"detected only", "Compactions inferred from a drop in context between calls, with no log record."},
		{"unapplied", "Compaction calls whose result the harness never applied to the context."},
		{"cost", "What the actor's compaction calls cost."},
		{"context", "The actor's context call by call, colored by band, with a marker at the first write."},
	}
}

// callDocs describe the columns of the per-call table of the Session screen.
func callDocs() []columnDoc {
	return []columnDoc{
		{"time", "When the API call was made."},
		{"model", "The model that answered the call."},
		{"kind", "Whether the call belongs to the main loop or to a compaction."},
		{"context", "The context the call sent: input + cache read + cache write, excluding output."},
		{"input", "Input tokens of the call that were not served from the cache."},
		{"cache read", "Input tokens of the call served from the prompt cache."},
		{"cache write", "Input tokens of the call written to the prompt cache."},
		{"output", "Output tokens the call produced."},
		{"cost", "What the call cost in the display currency."},
		{"requested tools", "The tools the call asked the harness to run, or not provided when the log does not record them."},
		{"context growth", "How much the context grew after a call that requested exactly one tool, up to the next call."},
		{"mark", "Notes on the call: the first write, compactions around it and rewrites."},
	}
}

// contextLegend is the legend of the Context table.
func contextLegend() []chrome.HelpGroup { return []chrome.HelpGroup{legendGroup("", contextDocs())} }

// callLegend is the legend of the Session screen: its per-call table and its tools table.
func callLegend() []chrome.HelpGroup {
	return []chrome.HelpGroup{legendGroup("per-call table", callDocs()), legendGroup("tools", sessionToolsLegend())}
}

// dispatchLegend is the legend of a table of dispatches.
func dispatchLegend() []chrome.HelpGroup { return []chrome.HelpGroup{legendGroup("", dispatchDocs())} }

// typeLegend is the legend of the by-type table.
func typeLegend() []chrome.HelpGroup { return []chrome.HelpGroup{legendGroup("", typeDocs())} }

// statusLegend is the legend of the by-status table.
func statusLegend() []chrome.HelpGroup { return []chrome.HelpGroup{legendGroup("", statusDocs())} }

// toolsLegend is the legend of the tools table and its context growth columns.
func toolsLegend() []chrome.HelpGroup {
	return []chrome.HelpGroup{legendGroup("tools", toolDocs()), legendGroup("context growth", growthDocs())}
}
