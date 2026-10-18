package main

// stack_reuse.go decides which catalog stack a run uses: the stack input when there is any, else
// the stack persisted with the run's workspace, else the default layer alone.
//
// Pre-scan: the stack is built here, before cobra parses subcommand flags, so the workspace of
// the run has to be found from the raw arguments. The scan is subcommand-aware. It first
// identifies the subcommand (the first argument that is not a global flag or a global flag's
// value, as cobra does) and then reads only the workspace-bearing flag that subcommand declares:
//
//	deploy, update, workflows, agents, hooks, render: --workspace
//	promote: --file    (workspace = nearest ancestor holding .mosaic/manifest.yaml)
//	transform: --path  (workspace = the path itself or its nearest such ancestor)
//	check-index: none  (validates catalogs, never reuses)
//
// Both --flag value and --flag=value are understood. render without --workspace has no
// workspace. This file holds no persistence logic of its own: reading, validating and workspace
// lookup are the stackconfig operations, and nothing here writes the manifest.

import (
	"errors"
	"strings"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/pathinput"
)

// workspaceFlag describes where a subcommand carries its workspace.
type workspaceFlag struct {
	name   string
	lookup bool // true: the value is a path inside the workspace, found by ancestor lookup
}

var subcommandWorkspaceFlags = map[string]workspaceFlag{
	"deploy":    {name: "--workspace"},
	"update":    {name: "--workspace"},
	"workflows": {name: "--workspace"},
	"agents":    {name: "--workspace"},
	"hooks":     {name: "--workspace"},
	"render":    {name: "--workspace"},
	"promote":   {name: "--file", lookup: true},
	"transform": {name: "--path", lookup: true},
}

// globalValueFlags are the global flags that take their value from the next argument.
var globalValueFlags = map[string]bool{
	"--mosaic-root":    true,
	"--catalog-folder": true,
	"--log-dir":        true,
}

// loadRunStack builds the catalog stack of the run and says where it came from. Only the
// stack-input origin is persisted by the write runs; a reused stack is never from input.
func loadRunStack(mosaicRoot string, args []string) (cat stack.Catalog, desc stackconfig.Description, origin stackconfig.Origin, err error) {
	in := scanStackInput(args)
	if !in.Given {
		stored, ok, rerr := persistedStackOf(args)
		if rerr != nil {
			return nil, stackconfig.Description{}, "", rerr
		}
		if ok {
			if err := stackconfig.Validate(stored, mosaicRoot); err != nil {
				return nil, stackconfig.Description{}, "", err
			}
			desc = stored.Description()
			in = stackInput{Layers: desc.UserLayers, IncludeDefault: desc.IncludeDefault, Given: true}
			cat, err = buildCatalogStack(mosaicRoot, in)
			return cat, desc, stackconfig.OriginPersisted, err
		}
	}
	cat, err = buildCatalogStack(mosaicRoot, in)
	return cat, stackDescription(in), originOf(in), err
}

// originOf is stack-input when the run gave stack input, otherwise the default layer alone.
func originOf(in stackInput) stackconfig.Origin {
	if in.Given {
		return stackconfig.OriginStackInput
	}
	return stackconfig.OriginDefault
}

// persistedStackOf returns the stack persisted with the workspace of the run, if any. A
// workspace whose manifest cannot be read yields no stack (the app layer reports that
// manifest itself); a persisted stack that is malformed stops the run.
func persistedStackOf(args []string) (stackconfig.Stored, bool, error) {
	workspace, ok := runWorkspace(args)
	if !ok {
		return stackconfig.Stored{}, false, nil
	}
	stored, ok, err := stackconfig.Read(workspace)
	if err != nil {
		if errors.Is(err, stackconfig.ErrStoredStackMalformed) {
			return stackconfig.Stored{}, false, err
		}
		return stackconfig.Stored{}, false, nil
	}
	return stored, ok, nil
}

// runWorkspace pre-scans args for the workspace of the run, per the rules in the file comment.
func runWorkspace(args []string) (string, bool) {
	sub, rest, found := splitSubcommand(args)
	if !found {
		return "", false
	}
	wf, declared := subcommandWorkspaceFlags[sub]
	if !declared {
		return "", false
	}
	value := lastFlagValue(rest, wf.name)
	if value == "" {
		return "", false
	}
	if wf.lookup {
		return stackconfig.FindWorkspace(value)
	}
	return value, true
}

// splitSubcommand returns the subcommand and the arguments after it. Global flags (and the
// values of the value-taking ones) before the subcommand are skipped.
func splitSubcommand(args []string) (sub string, rest []string, found bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case globalValueFlags[arg]:
			i++
		case strings.HasPrefix(arg, "-"):
		default:
			return arg, args[i+1:], true
		}
	}
	return "", nil, false
}

// lastFlagValue returns the value of the last occurrence of name in either form, unquoted.
func lastFlagValue(args []string, name string) string {
	var value string
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == name && i+1 < len(args):
			value = args[i+1]
			i++
		case strings.HasPrefix(arg, name+"="):
			value = strings.TrimPrefix(arg, name+"=")
		}
	}
	return pathinput.Unquote(strings.TrimSpace(value))
}
