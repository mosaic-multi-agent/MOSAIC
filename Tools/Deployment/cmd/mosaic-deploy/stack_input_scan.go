package main

import (
	"path/filepath"
	"strconv"
	"strings"

	"mosaic-deploy/internal/pathinput"
)

const (
	catalogFolderFlag         = "--catalog-folder"
	includeDefaultCatalogFlag = "--include-default-catalog"
)

// stackInput is the catalog-stack request collected from the raw arguments.
//
// Layers are the user catalog roots in argument order (index 0 is the highest layer), made
// absolute and cleaned. IncludeDefault is the last valid --include-default-catalog value.
// Given is true when at least one --catalog-folder value or one valid
// --include-default-catalog occurrence was seen.
type stackInput struct {
	Layers         []string
	IncludeDefault bool
	Given          bool
}

// scanStackInput pre-scans args for the stack flags. It is separate from scanGlobalFlags,
// whose signature is unchanged. Both --flag value and --flag=value forms are understood.
// --include-default-catalog is a bool flag: its bare form never consumes the next token, and
// an =value that cobra would reject is ignored (cobra then reports the usage error).
func scanStackInput(args []string) stackInput {
	var in stackInput
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == catalogFolderFlag && i+1 < len(args):
			in.addLayer(args[i+1])
			i++
		case strings.HasPrefix(arg, catalogFolderFlag+"="):
			in.addLayer(strings.TrimPrefix(arg, catalogFolderFlag+"="))
		case arg == includeDefaultCatalogFlag:
			in.IncludeDefault, in.Given = true, true
		case strings.HasPrefix(arg, includeDefaultCatalogFlag+"="):
			v, err := strconv.ParseBool(strings.TrimPrefix(arg, includeDefaultCatalogFlag+"="))
			if err == nil {
				in.IncludeDefault, in.Given = v, true
			}
		}
	}
	return in
}

// addLayer records one --catalog-folder value as an absolute, cleaned user layer root.
func (in *stackInput) addLayer(raw string) {
	value := pathinput.Unquote(strings.TrimSpace(raw))
	if value == "" {
		return
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		abs = filepath.Clean(value)
	}
	in.Layers = append(in.Layers, abs)
	in.Given = true
}
