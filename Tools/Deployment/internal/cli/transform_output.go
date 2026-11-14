package cli

// transform_output.go renders the result of the transform subcommand for the terminal and
// as JSON, and maps it to an exit code.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// renderTransformOutput writes the transform result and returns the exit code. With format
// "json" the result is one JSON document on out, also when the run was cancelled. A service
// error goes to errOut; a cancelled run additionally says that nothing was written.
func renderTransformOutput(out, errOut io.Writer, format string, result app.TransformHarnessResult, svcErr error) int {
	asJSON := strings.EqualFold(format, "json")
	if svcErr != nil {
		hasResult := result.Cancelled || result.Workspace != "" || len(result.Files) > 0
		switch {
		case asJSON && hasResult:
			encodeTransformJSON(out, result)
		case !asJSON && hasResult:
			renderTransformHuman(out, result)
		}
		fmt.Fprintf(errOut, "error: %v\n", svcErr)
		if msg := transformCancelMessage(result, svcErr); msg != "" {
			fmt.Fprintln(errOut, msg)
		}
		return ExitFailure
	}
	if asJSON {
		encodeTransformJSON(out, result)
	} else {
		renderTransformHuman(out, result)
	}
	return transformExitCode(result)
}

func encodeTransformJSON(out io.Writer, result app.TransformHarnessResult) {
	_ = json.NewEncoder(out).Encode(result)
}

// transformCancelMessage describes a cancelled run; it is empty for any other error.
func transformCancelMessage(result app.TransformHarnessResult, svcErr error) string {
	switch {
	case errors.Is(svcErr, app.ErrPlanNotConfirmed):
		return "transform plan was not confirmed: nothing was written (use --yes to confirm the plan without prompting)"
	case result.Cancelled, errors.Is(svcErr, app.ErrModelSelectionCancelled),
		errors.Is(svcErr, app.ErrSelectionCancelled):
		return "transform cancelled: nothing was written"
	}
	return ""
}

// transformExitCode maps a finished run to an exit code: a failed file is a failure; items
// the user has to follow up on (agents without a model, tools left unmapped, hook bundles
// not carried) give ExitWithSkips.
func transformExitCode(result app.TransformHarnessResult) int {
	if result.Failed > 0 {
		return ExitFailure
	}
	followUp := len(result.UnresolvedModels) > 0
	for _, f := range result.Files {
		if f.Status == app.StatusFailed {
			return ExitFailure
		}
		if len(f.UnmappedTools) > 0 || hookNotCarried(f) {
			followUp = true
		}
	}
	if followUp {
		return ExitWithSkips
	}
	return ExitSuccess
}

// hookNotCarried reports whether f is a hook bundle skipped because the target cannot carry it.
func hookNotCarried(f app.TransformFileOutcome) bool {
	if f.Kind != domain.ArtifactHook || f.Status != app.StatusSkipped {
		return false
	}
	switch f.Skip {
	case app.SkipUnsupported, app.SkipNoTargetVariant, app.SkipSharedHooksFolder:
		return true
	}
	return false
}

// renderTransformHuman writes the transform plan and summary to out. The plan listing is the
// same for a real run, a dry run and a plan that was not confirmed.
func renderTransformHuman(out io.Writer, result app.TransformHarnessResult) {
	title := "transform plan"
	if result.DryRun {
		title = "dry run: transform plan (nothing is written)"
	}
	fmt.Fprintf(out, "%s\nsource: %s -> %s\nworkspace: %s\n",
		title, result.SourceHarnessID, result.TargetHarnessID, result.Workspace)
	if result.SourceAgentsDir != "" {
		fmt.Fprintf(out, "agents folder: %s\n", result.SourceAgentsDir)
	}
	for _, f := range result.Files {
		fmt.Fprintln(out, transformFileLine(f))
		writeTransformFileDetails(out, f)
	}
	writeTransformChanges(out, result)
	writeTransformDroppedEntries(out, result.DroppedEntries)
	fmt.Fprintf(out, "transformed: %d  skipped: %d  failed: %d%s\n",
		result.Transformed, result.Skipped, result.Failed, transformChangeCounts(result))
	if len(result.UnresolvedModels) > 0 {
		fmt.Fprintf(out, "agents written without a model (set one in the file): %s\n",
			strings.Join(result.UnresolvedModels, ", "))
	}
	for _, n := range result.Notes {
		fmt.Fprintf(out, "note: %s\n", n.Message)
		for _, p := range n.Paths {
			fmt.Fprintf(out, "  %s\n", p)
		}
	}
	if result.TodoPath != "" {
		fmt.Fprintf(out, "todo: %s\n", result.TodoPath)
	}
}

// transformFileLine is the one-line plan entry of an item: status, source and destination,
// planned action, and the reason for a skipped or failed item.
func transformFileLine(f app.TransformFileOutcome) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %s [%s] ", f.Status, f.Action)
	switch {
	case f.SourcePath != "" && f.DestinationPath != "":
		fmt.Fprintf(&b, "%s -> %s", f.SourcePath, f.DestinationPath)
	case f.SourcePath != "":
		b.WriteString(f.SourcePath)
	case f.DestinationPath != "":
		b.WriteString(f.DestinationPath)
	default:
		b.WriteString(f.Key)
	}
	if f.Reason != "" {
		fmt.Fprintf(&b, " - %s", f.Reason)
	}
	return b.String()
}

// writeTransformFileDetails lists the tools of a transformed agent. The lines carry no file
// name so the plan listing stays one line per item.
func writeTransformFileDetails(out io.Writer, f app.TransformFileOutcome) {
	if f.Status != app.StatusTransformed {
		return
	}
	if len(f.Tools) > 0 {
		fmt.Fprintf(out, "    tools: %s\n", strings.Join(f.Tools, ", "))
	}
	if len(f.CustomTools) > 0 {
		fmt.Fprintf(out, "    custom tools: %s\n", strings.Join(f.CustomTools, ", "))
	}
	if len(f.UnmappedTools) > 0 {
		fmt.Fprintf(out, "    tools without a target equivalent: %s\n", strings.Join(f.UnmappedTools, ", "))
	}
}
