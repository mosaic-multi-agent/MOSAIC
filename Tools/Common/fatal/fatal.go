// Package fatal reports fatal failures and panics of a MOSAIC tool process:
// screen report, failure file next to the executable, one CLI stderr line naming
// that file, Enter pause on a Windows TUI start, and process exit.
//
// Wiring recipe for a tool's main package:
//
//  1. Decide the mode first (ModeTUI when the tool starts its TUI, ModeCLI
//     otherwise, and always ModeCLI for --version).
//  2. r := fatal.Install(fatal.Config{Tool: ..., Version: ..., Mode: ...}), then
//     defer fatal.Recover() as the next statement of main.
//  3. Replace every os.Exit with a Reporter call: Fail for an unexpected failure
//     (message is the text the tool printed before), Exit for success, documented
//     outcome codes and an interrupted TUI, ExitCLI for the result code of a CLI
//     frontend. Nothing else may end the process.
//  4. In CLI mode write all stderr output through r.Stderr(); ExitCLI(1) then
//     reports everything written there as the failure message.
//  5. Build the TUI program with r.NewProgram (a drop-in for tea.NewProgram) and
//     read the final model through UnwrapModel.
//  6. Defer fatal.Recover() as the last-registered deferred call of every
//     goroutine the tool starts itself, so a panic there is reported too.
//
// CLI stderr rule: when a CLI run ends with an unexpected failure (exit code 1) or
// a panic and the failure file was actually written, exactly one extra line,
// "failure report written to <path>", follows the tool's existing stderr output.
// Nothing else on stderr changes. No line is added when the file could not be
// written, for documented outcome codes, or on success. Stdout and exit codes are
// never touched; a panic always ends with exit code 1.
//
// A failure file is named <tool>-failure-<UTC time>-<pid>.txt and lives in
// Config.FailureDir (by default the folder of the running executable). It holds
// the time, the tool and its version, the command line, the message and, for a
// panic, the stack trace. When the folder cannot be resolved or written the file
// is skipped silently; the failure itself is always shown. On a Windows TUI start
// the report is followed by PausePrompt and a wait for Enter.
package fatal

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// PausePrompt is printed, without a trailing newline, before waiting for Enter.
const PausePrompt = "Press Enter to exit..."

// Mode selects whether the process runs a TUI or a command line frontend.
type Mode int

const (
	// ModeCLI never pauses.
	ModeCLI Mode = iota
	// ModeTUI pauses after a report when the OS is Windows.
	ModeTUI
)

// Config is everything the reporter needs. The zero value of every optional
// field uses the real process.
type Config struct {
	Tool    string // binary name, e.g. "mosaic-deploy"
	Version string // the tool's ToolVersion
	Mode    Mode

	Args       []string               // default os.Args
	GOOS       string                 // default runtime.GOOS
	Stdin      io.Reader              // default os.Stdin; read for the pause
	Stderr     io.Writer              // default os.Stderr
	Exit       func(code int)         // default os.Exit
	Now        func() time.Time       // default time.Now
	FailureDir func() (string, error) // default: folder of os.Executable()
}

var (
	installedMu sync.Mutex
	installed   *Reporter
)

// Install creates the process reporter from cfg and makes it the target of Recover.
// A later Install replaces the earlier reporter.
func Install(cfg Config) *Reporter {
	r := newReporter(applyDefaults(cfg))
	installedMu.Lock()
	installed = r
	installedMu.Unlock()
	return r
}

// Reset removes the installed reporter (tests). Recover is a no-op afterwards.
func Reset() {
	installedMu.Lock()
	installed = nil
	installedMu.Unlock()
}

func currentReporter() *Reporter {
	installedMu.Lock()
	defer installedMu.Unlock()
	return installed
}

func applyDefaults(cfg Config) Config {
	if cfg.Args == nil {
		cfg.Args = os.Args
	}
	if cfg.GOOS == "" {
		cfg.GOOS = runtime.GOOS
	}
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.Exit == nil {
		cfg.Exit = os.Exit
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.FailureDir == nil {
		cfg.FailureDir = executableDir
	}
	return cfg
}

func executableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}
