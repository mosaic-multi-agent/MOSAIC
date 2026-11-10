package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"mosaic-log-analyzer/internal/app"
)

// newExportCommand builds the export subcommand. It reads the shared --path and
// --run flags through the supplied getters and never prompts.
func newExportCommand(opts Options, exitCode *int, pathFlag, runFlag func() string) *cobra.Command {
	var (
		outFlag   string
		forceFlag bool
	)
	cmd := &cobra.Command{
		Use:           "export",
		Short:         "Write a content-free run digest file",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if outFlag == "" {
				fmt.Fprintln(opts.Stderr, "error: --out is required for the export command")
				*exitCode = ExitUsage
				return nil
			}
			res, err := opts.Service.Export(context.Background(), app.ExportRequest{
				ExplicitPath: pathFlag(),
				RunID:        runFlag(),
				OutPath:      outFlag,
				Overwrite:    forceFlag,
			})
			if err != nil {
				fmt.Fprintf(opts.Stderr, "error: %v\n", err)
				*exitCode = ExitFailure
				return nil
			}
			*exitCode = reportExport(opts, res, runFlag())
			return nil
		},
	}
	cmd.Flags().StringVar(&outFlag, "out", "", "digest file to write (must end in .digest.json)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "replace an existing digest file")
	return cmd
}

// reportExport prints the outcome of an export and returns the exit code.
func reportExport(opts Options, res app.ExportResult, runID string) int {
	switch res.Status {
	case app.ExportWritten:
		fmt.Fprintln(opts.Stdout, exportMessage(res))
		return ExitSuccess
	case app.ExportInvalidPath:
		fmt.Fprintf(opts.Stderr, "error: --out %q must end in .digest.json\n", res.OutPath)
		return ExitUsage
	case app.ExportFileExists:
		fmt.Fprintf(opts.Stderr, "error: %s already exists; use --force to replace it\n", res.OutPath)
		return ExitUsage
	case app.ExportRunNotFound:
		fmt.Fprintf(opts.Stderr, "run %s was not found in the source; nothing written\n", runID)
		return ExitNoData
	}
	switch res.Outcome {
	case app.OutcomeSourceNotFound:
		return ExitUsage
	case app.OutcomeSourceUnusable:
		return ExitUnusable
	}
	return ExitNoData
}

func exportMessage(res app.ExportResult) string {
	noun := "runs"
	if res.RunsWritten == 1 {
		noun = "run"
	}
	msg := fmt.Sprintf("exported %d %s", res.RunsWritten, noun)
	if res.UnattributableWritten {
		msg += " and the unattributable bucket"
	}
	return msg + " to " + res.OutPath
}
