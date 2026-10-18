package cli

import "github.com/spf13/cobra"

// registerStackFlags declares the catalog-stack flags as persistent flags on root so that
// cobra accepts them on every subcommand. The values themselves are read by the composition
// root's pre-scan, which builds the stack before cobra runs; cobra only validates them here.
//
// The layers are named by repeating --catalog-folder; the first occurrence is the highest
// layer. A run that names a user catalog leaves the default catalog out unless
// --include-default-catalog is given, which places it as the bottom layer.
func registerStackFlags(root *cobra.Command) {
	flags := root.PersistentFlags()
	flags.StringArray("catalog-folder", nil,
		"User catalogue directory forming a stack layer; repeat for several layers. "+
			"The first occurrence is the highest layer (it takes precedence); each later one sits below it. "+
			"Without --include-default-catalog the default catalogue (<mosaic-root>/Catalog) is left out. "+
			"Any stack flag replaces the stack persisted with the workspace (never merged); "+
			"without stack flags a run reuses the workspace's persisted stack, or the default catalogue alone when none is persisted")
	flags.Bool("include-default-catalog", false,
		"Add the default catalogue (<mosaic-root>/Catalog) as the bottom layer, below every --catalog-folder. "+
			"Given alone, it selects the default catalogue alone and replaces a persisted user-catalogue stack "+
			"(the way back to a default-only stack)")
	// A bare --include-default-catalog must not consume the following token.
	flags.Lookup("include-default-catalog").NoOptDefVal = "true"
}
