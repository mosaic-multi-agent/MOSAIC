// Command mosaic-deploy is the entry point for the MOSAIC deployment tool.
//
// When a subcommand is present in the arguments (deploy or update), the CLI adapter handles
// the invocation non-interactively. When no subcommand is present and a terminal is attached
// to both stdin and stdout, the TUI frontend is launched instead. This is also what happens
// when the binary is double-clicked from Explorer: Windows attaches a fresh console with a
// real stdin/stdout, so isatty is true and the TUI takes over.
//
// Dependency construction (catalog, registry, manifest, config, logging, todo) is done here
// once, before dispatching to a frontend. Neither the CLI nor TUI packages construct their
// own infrastructure; they receive a fully-wired app.Service.
package main

import (
	"os"
	"strings"

	"mosaic-common/fatal"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/harness/registry"
	"mosaic-deploy/internal/pathinput"

	// Import built-in harnesses so their init() functions run and register
	// themselves with the package-level registry before Discover is called.
	_ "mosaic-deploy/internal/harness/builtin/claudecode"
	_ "mosaic-deploy/internal/harness/builtin/codex"
	_ "mosaic-deploy/internal/harness/builtin/ghcpcli"
	_ "mosaic-deploy/internal/harness/builtin/opencode"
	_ "mosaic-deploy/internal/harness/builtin/opencodev2"
	_ "mosaic-deploy/internal/harness/builtin/vscodeghcp"

	// Import the translator wiring package so that all format translators (CodexTOML
	// and any future ones) register themselves via their init() functions before any
	// LookupString call at runtime. This is the non-test owner of translator registration;
	// the T2.8a test in main_test.go guards that both this import and the Codex harness
	// import above are present.
	_ "mosaic-common/agentformat/all"
)

const ToolVersion = "1.14.0"

// main hands the whole run to runTool, which ends the process through the fatal reporter.
func main() {
	runTool(fatal.Config{}, os.Args[1:], defaultExitSeams())
}

// discoverHarnesses discovers the available harnesses. The registry options come from
// registryOptions so the ToolMappings hook re-reads both config files at every registry
// build, and a registry rebuilt after a saved custom tool answer applies it in the same process.
func discoverHarnesses(
	mosaicRoot string, allowExternal bool,
	toolStore config.ToolConfigStore, userStore config.UserConfigStore,
	toolCfg config.ToolConfig, userCfg config.UserConfig,
	res injections.Result,
) (registry.Registry, error) {
	return registry.DiscoverForStack(registryOptions(
		mosaicRoot, allowExternal, toolStore, userStore, toolCfg, userCfg,
	), res)
}

// scanGlobalFlags does a minimal pre-scan of args for the global flags that
// influence dependency wiring: --mosaic-root, --catalog-folder, --log-dir,
// and --allow-external. cobra will re-parse all flags when cli.Run executes;
// this pre-scan exists only so that main.go can construct app.New(deps) before
// handing control to cli.Run.
//
// catalogFolder is the raw user-supplied value, or "" when the flag is absent.
// Defaulting to {mosaic-root}/Catalog is done by resolveCatalogFolder, not here.
//
// logDir is the raw user-supplied value, or "" when the flag is absent, in
// which case the logger writes to its default location.
//
// The scan understands both --flag value and --flag=value forms. It does not
// validate other flags — unknown flags are left for cobra to handle.
func scanGlobalFlags(args []string) (mosaicRoot, catalogFolder, logDir string, allowExternal bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--mosaic-root" && i+1 < len(args):
			mosaicRoot = pathinput.Unquote(strings.TrimSpace(args[i+1]))
			i++
		case strings.HasPrefix(arg, "--mosaic-root="):
			mosaicRoot = pathinput.Unquote(strings.TrimSpace(strings.TrimPrefix(arg, "--mosaic-root=")))
		case arg == "--catalog-folder" && i+1 < len(args):
			catalogFolder = pathinput.Unquote(strings.TrimSpace(args[i+1]))
			i++
		case strings.HasPrefix(arg, "--catalog-folder="):
			catalogFolder = pathinput.Unquote(strings.TrimSpace(strings.TrimPrefix(arg, "--catalog-folder=")))
		case arg == "--log-dir" && i+1 < len(args):
			logDir = pathinput.Unquote(strings.TrimSpace(args[i+1]))
			i++
		case strings.HasPrefix(arg, "--log-dir="):
			logDir = pathinput.Unquote(strings.TrimSpace(strings.TrimPrefix(arg, "--log-dir=")))
		case arg == "--allow-external":
			allowExternal = true
		}
	}
	return
}

// buildPreAnswers pre-scans args for --selections and, when present, parses the file
// into a PreAnswers value for the non-interactive Interaction. The scan follows the
// same --flag value / --flag=value form as scanGlobalFlags. When the flag is absent,
// or when the file cannot be read or parsed, an empty PreAnswers is returned so that
// cobra's own flag validation and exit-code logic remain the authoritative handler for
// bad --selections values.
func buildPreAnswers(args []string) cli.PreAnswers {
	var selectionsPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--selections" && i+1 < len(args):
			selectionsPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--selections="):
			selectionsPath = strings.TrimPrefix(arg, "--selections=")
		}
	}
	if selectionsPath == "" {
		return cli.PreAnswers{}
	}
	pa, err := cli.PreAnswersFromSelectionsFile(selectionsPath)
	if err != nil {
		// Return empty so cobra handles the error at flag-parse time with the correct exit code.
		return cli.PreAnswers{}
	}
	return pa
}
