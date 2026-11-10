package cli

import "github.com/spf13/cobra"

// registerPromoteTargetFlag declares promote's --target-catalog flag, which names the stack
// layer (by catalog id) that receives a new item. The service enforces when it is required.
func registerPromoteTargetFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "target-catalog", "",
		"Catalog id of the stack layer that receives a new agent (required when promoting a new agent non-interactively into a stack of two or more layers)")
}
