package cli

// transform_cmd.go builds the transform subcommand: it moves the whole deployment of a
// workspace from one harness to another.

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/pathinput"
)

// newTransformCmd returns the transform subcommand. It stores the exit code of a run in
// exitCode.
func newTransformCmd(ctx context.Context, svc app.Service, out, errOut io.Writer, exitCode *int) *cobra.Command {
	var (
		harness       string
		targetHarness string
		workspace     string
		targetModel   string
		modelMap      []string
		overwrite     bool
		dryRun        bool
		autoConfirm   bool
		output        string
	)

	cmd := &cobra.Command{
		Use:   "transform",
		Short: "Move a workspace's deployment from one harness to another",
		Long: `Move the deployment of a workspace from a source harness to a target harness.

The input is the workspace root, as for deploy. Transform plans the whole deployment found
there (agents, skills and hook bundles), shows the plan, and writes the target harness's
copies only after the plan is confirmed. Use --dry-run to see the plan without writing
anything, and --yes to confirm the plan without prompting.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			required := []struct{ flag, value string }{
				{"--harness", harness},
				{"--target-harness", targetHarness},
				{"--workspace", strings.TrimSpace(workspace)},
			}
			for _, r := range required {
				if r.value == "" {
					fmt.Fprintf(errOut, "error: %s is required for the transform subcommand\n", r.flag)
					*exitCode = ExitUsage
					return nil
				}
			}

			req := app.TransformHarnessRequest{
				SourceHarnessID: harness,
				TargetHarnessID: targetHarness,
				Workspace:       pathinput.Unquote(strings.TrimSpace(workspace)),
				AutoConfirmPlan: autoConfirm,
				TargetModel:     targetModel,
				Overwrite:       overwrite,
				DryRun:          dryRun,
			}
			if len(modelMap) > 0 {
				parsed, parseErr := parseModelMap(modelMap)
				if parseErr != nil {
					fmt.Fprintf(errOut, "error: %s\n", parseErr)
					*exitCode = ExitUsage
					return nil
				}
				req.ModelMap = parsed
				req.SkipAll = map[domain.QuestionID]bool{domain.QTransformTargetModel: true}
			}

			result, svcErr := svc.TransformHarness(ctx, req)
			out := withDocStackInEffect(out, svc, output)
			*exitCode = renderTransformOutput(out, errOut, output, result, svcErr)
			return nil
		},
	}

	cmd.Flags().StringVar(&harness, "harness", "", "Source harness ID (required)")
	cmd.Flags().StringVar(&targetHarness, "target-harness", "", "Target harness ID (required)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace root whose deployment is moved (required)")
	cmd.Flags().StringArrayVar(&modelMap, "model-map", nil,
		"Map a source model to a target model as sourceModel=targetModel; repeat once per "+
			"source model. Use an empty source (=targetModel) for agents with no model set. "+
			"Source models not mapped here fall back to --target-model, and are left empty when "+
			"--target-model is absent.")
	cmd.Flags().StringVar(&targetModel, "target-model", "",
		"Default target model for source models not covered by --model-map; absent means "+
			"those agents' model field is left empty")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite an existing destination file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the plan and outcomes without writing any file")
	cmd.Flags().BoolVar(&autoConfirm, "yes", false, "Confirm the transform plan without prompting")
	cmd.Flags().StringVar(&output, "output", "", "Output format (json)")
	return cmd
}
