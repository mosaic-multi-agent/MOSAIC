package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// otelFileEnvVar names the environment variable that holds the Copilot OTel
// file path when the command is given no file argument.
const otelFileEnvVar = "COPILOT_OTEL_FILE_EXPORTER_PATH"

// newIngestOtelCommand builds the ingest-otel subcommand. It reads the shared
// --path and --run flags through the supplied getters and never prompts.
func newIngestOtelCommand(opts Options, exitCode *int, pathFlag, runFlag func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "ingest-otel [otel-file]",
		Short: "Ingest the Copilot OTel file into a run's logs (recovery ingest)",
		Long: "Runs a final ingest of the whole Copilot OTel file into the run named by --run.\n" +
			"The file is the argument, or the value of " + otelFileEnvVar + " when omitted.",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, args []string) error {
			*exitCode = runIngestOtel(opts, args, pathFlag(), runFlag())
			return nil
		},
	}
}

// runIngestOtel performs the command and returns its exit code.
func runIngestOtel(opts Options, args []string, path, runID string) int {
	if runID == "" {
		fmt.Fprintln(opts.Stderr, "error: --run is required for the ingest-otel command")
		return ExitUsage
	}
	exportPath := otelExportPath(opts, args)
	if exportPath == "" {
		fmt.Fprintf(opts.Stderr, "error: no OTel file: pass it as an argument or set %s\n", otelFileEnvVar)
		return ExitUsage
	}
	out, err := opts.Service.IngestOTel(context.Background(), app.IngestRequest{
		ExplicitPath: path, RunID: runID, ExportPath: exportPath,
	})
	if err != nil {
		fmt.Fprintf(opts.Stderr, "error: %v\n", err)
		return ExitFailure
	}
	return reportIngest(opts, out, runID)
}

// otelExportPath returns the argument, else the environment variable, else "".
func otelExportPath(opts Options, args []string) string {
	if len(args) > 0 && args[0] != "" {
		return args[0]
	}
	if opts.Getenv == nil {
		return ""
	}
	return opts.Getenv(otelFileEnvVar)
}

// reportIngest prints the outcome of an ingest and returns the exit code.
func reportIngest(opts Options, out app.IngestOutcome, runID string) int {
	switch out.Outcome {
	case app.OutcomeSourceNotFound:
		fmt.Fprintln(opts.Stderr, "error: no logs source found; pass --path")
		return ExitUsage
	case app.OutcomeSourceUnusable:
		fmt.Fprintln(opts.Stderr, "error: the source is not a logs tree with run folders to ingest into")
		return ExitUnusable
	case app.OutcomeNoData:
		fmt.Fprintln(opts.Stderr, "error: the source holds no runs")
		return ExitNoData
	}
	if !out.RunFound {
		fmt.Fprintf(opts.Stderr, "run %s was not found in the source; nothing ingested\n", runID)
		return ExitNoData
	}
	for _, d := range out.Result.Diagnostics {
		fmt.Fprintf(opts.Stderr, "%s: %s\n", d.Kind, d.Message)
	}
	switch out.Result.Status {
	case domain.IngestDone:
		r := out.Result
		fmt.Fprintf(opts.Stdout, "run %s: %d records written, %d finalised as unattributed, %d pending\n",
			runID, r.RecordsWritten, r.Finalized, r.Pending)
		return ExitSuccess
	case domain.IngestFileMissing:
		return ExitUsage
	case domain.IngestLockHeld, domain.IngestUnknownState:
		return ExitIngestSkipped
	}
	return ExitFailure
}
