// Package codexsetup checks the Codex configuration for problems that would
// silently break a Codex run: an untrusted workspace and a missing contact-user
// MCP server. It reads the configuration read-only and never writes a file.
package codexsetup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"

	commonharness "mosaic-common/harness"
)

// TrustStatus classifies the workspace trust check.
type TrustStatus string

const (
	TrustTrusted       TrustStatus = "trusted"
	TrustUntrusted     TrustStatus = "untrusted"
	TrustIndeterminate TrustStatus = "indeterminate"
)

// ServerStatus classifies the contact-user server check.
type ServerStatus string

const (
	ServerConfigured    ServerStatus = "configured"
	ServerMissing       ServerStatus = "missing"
	ServerIndeterminate ServerStatus = "indeterminate"
)

// Result is the outcome of the two Codex setup checks.
type Result struct {
	ConfigPath        string // home config.toml consulted; "" if unresolvable
	Trust             TrustStatus
	TrustReason       string // non-empty only for TrustIndeterminate
	ContactUser       ServerStatus
	ContactUserReason string // non-empty only for ServerIndeterminate
}

// Checker evaluates the Codex setup for a working directory.
type Checker struct {
	getenv          func(string) string
	userHomeDir     func() (string, error)
	caseInsensitive bool
}

// Option configures a Checker (test seams).
type Option func(*Checker)

// WithGetenv replaces the environment lookup.
func WithGetenv(getenv func(string) string) Option {
	return func(c *Checker) { c.getenv = getenv }
}

// WithUserHomeDir replaces the home directory lookup.
func WithUserHomeDir(home func() (string, error)) Option {
	return func(c *Checker) { c.userHomeDir = home }
}

// WithCaseInsensitive sets case-insensitive path comparison.
func WithCaseInsensitive(ci bool) Option {
	return func(c *Checker) { c.caseInsensitive = ci }
}

// NewChecker returns a checker with production defaults.
func NewChecker(opts ...Option) *Checker {
	c := &Checker{
		getenv:          os.Getenv,
		userHomeDir:     os.UserHomeDir,
		caseInsensitive: runtime.GOOS == "windows",
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// contactUserServer is the MCP server name MOSAIC deployments configure.
const contactUserServer = commonharness.CodexContactUserServer

// Check inspects the Codex configuration for workDir read-only.
//
// A missing home configuration decides both checks (untrusted, no server); an
// unreadable or malformed one leaves both indeterminate. The project
// configuration is consulted only when the workspace is trusted, because Codex
// ignores it otherwise.
func (c *Checker) Check(workDir string) Result {
	var res Result
	homeDir := c.codexHome()
	if homeDir == "" {
		reason := "cannot resolve the Codex home directory (CODEX_HOME unset and no user home directory)"
		return indeterminate(res, reason)
	}
	res.ConfigPath = filepath.Join(homeDir, "config.toml")

	home, exists, err := readConfig(res.ConfigPath)
	if err != nil {
		return indeterminate(res, err.Error())
	}
	if !exists {
		res.Trust = TrustUntrusted
		res.ContactUser = ServerMissing
		return res
	}

	res.Trust = c.trustFor(home, workDir)
	res.ContactUser = ServerMissing
	if serverEnabled(home) {
		res.ContactUser = ServerConfigured
		return res
	}
	if res.Trust != TrustTrusted {
		return res
	}
	projectPath := filepath.Join(workDir, ".codex", "config.toml")
	project, projExists, err := readConfig(projectPath)
	if err != nil {
		res.ContactUser = ServerIndeterminate
		res.ContactUserReason = err.Error()
		return res
	}
	if projExists && serverEnabled(project) {
		res.ContactUser = ServerConfigured
	}
	return res
}

func indeterminate(res Result, reason string) Result {
	res.Trust = TrustIndeterminate
	res.TrustReason = reason
	res.ContactUser = ServerIndeterminate
	res.ContactUserReason = reason
	return res
}

// codexHome returns $CODEX_HOME, else <user home>/.codex, else "".
func (c *Checker) codexHome() string {
	if h := c.getenv("CODEX_HOME"); h != "" {
		return h
	}
	home, err := c.userHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".codex")
}

// readConfig parses a TOML file. A file that does not exist is reported as
// exists=false without error.
func readConfig(path string) (cfg map[string]any, exists bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, fmt.Errorf("cannot read %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, true, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return cfg, true, nil
}

// serverEnabled reports whether the config declares an enabled contact-user
// server table.
func serverEnabled(cfg map[string]any) bool {
	servers, _ := cfg["mcp_servers"].(map[string]any)
	entry, ok := servers[contactUserServer].(map[string]any)
	if !ok {
		return false
	}
	if enabled, ok := entry["enabled"].(bool); ok && !enabled {
		return false
	}
	return true
}

// trustFor finds the nearest [projects.*] entry at or above workDir; trusted
// only when that entry's trust_level is "trusted".
func (c *Checker) trustFor(cfg map[string]any, workDir string) TrustStatus {
	projects, _ := cfg["projects"].(map[string]any)
	entries := make(map[string]any, len(projects))
	for k, v := range projects {
		entries[c.normalise(k)] = v
	}
	cur := c.normalise(workDir)
	for {
		if v, ok := entries[cur]; ok {
			entry, _ := v.(map[string]any)
			if level, _ := entry["trust_level"].(string); level == "trusted" {
				return TrustTrusted
			}
			return TrustUntrusted
		}
		parent := path.Dir(cur)
		if parent == cur || parent == "." {
			return TrustUntrusted
		}
		cur = parent
	}
}

// normalise unifies separators, trailing separators and (when configured) case.
func (c *Checker) normalise(p string) string {
	p = path.Clean(strings.ReplaceAll(p, `\`, "/"))
	if c.caseInsensitive {
		p = strings.ToLower(p)
	}
	return p
}
