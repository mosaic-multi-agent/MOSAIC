package main

import "mosaic-deploy/internal/app/stackconfig"

// tuiLaunchStack maps the command-line arguments to the stack the TUI session starts from:
// the stack description and whether the arguments gave any stack input.
func tuiLaunchStack(args []string) (stackconfig.Description, bool) {
	in := scanStackInput(args)
	return stackDescription(in), in.Given
}
