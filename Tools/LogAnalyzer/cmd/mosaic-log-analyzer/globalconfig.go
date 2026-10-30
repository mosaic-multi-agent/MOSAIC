package main

import (
	"fmt"
	"os"

	"mosaic-log-analyzer/internal/userconfig"
)

// globalFileWriter is a store that can write its global file on first start.
type globalFileWriter interface {
	EnsureGlobalFile() (created bool, err error)
}

// globalConfigDir returns the user config folder for LogAnalyzer, or "" when the
// operating system has none (the global level is then unavailable).
func globalConfigDir() string {
	dir, err := userconfig.Dir(os.UserConfigDir)
	if err != nil {
		return ""
	}
	return dir
}

// ensureGlobalFiles writes each store's commented template to the global folder
// when absent. A failure is reported on stderr and never stops the program.
func ensureGlobalFiles(stores ...globalFileWriter) {
	for _, s := range stores {
		if _, err := s.EnsureGlobalFile(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not create global config file: %v\n", err)
		}
	}
}
