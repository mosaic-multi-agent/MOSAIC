package opencodev2

// Scope enumeration, the probe seam and per-document inspection. V2 reads user
// configuration from two files (opencode.json, opencode.jsonc) and two plugin
// folders (plugins/, plugin/); only a non-empty plugin registration key or a
// non-empty plugin folder can register a hook that rewrites the intercepted
// call. MCP configuration never counts.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mosaic-agent-test/internal/domain"
)

// Scope names, user scopes lowest precedence first.
const (
	scopeUserConfig       = "user-config"
	scopeUserConfigJSONC  = "user-config-jsonc"
	scopeUserPlugins      = "user-plugins"
	scopeUserPluginLegacy = "user-plugin-legacy"
	scopeSandbox          = "sandbox"
)

// userScopeFiles lists the user scopes by name with their path relative to the
// user configuration directory and whether the document is a plugin folder.
var userScopeFiles = []struct {
	name   string
	rel    string
	folder bool
}{
	{scopeUserConfig, "opencode.json", false},
	{scopeUserConfigJSONC, "opencode.jsonc", false},
	{scopeUserPlugins, "plugins", true},
	{scopeUserPluginLegacy, "plugin", true},
}

// Scopes enumerates the configuration scopes V2 merges, lowest precedence
// first. With a real sandbox (non-empty ControlDir) the user scopes live at the
// relocated paths and are in the run's control; without one they are the real
// locations resolved from XDG_CONFIG_HOME or the home directory.
func Scopes(s domain.Sandbox) []domain.ConfigScope {
	dir, relocated := userConfigDir(), false
	if s.ControlDir != "" {
		dir, relocated = UserConfigDir(s), true
	}
	scopes := make([]domain.ConfigScope, 0, len(userScopeFiles)+1)
	for _, u := range userScopeFiles {
		scopes = append(scopes, domain.ConfigScope{
			Name:       u.name,
			Path:       filepath.Join(dir, u.rel),
			InSandbox:  relocated,
			Isolatable: relocated,
		})
	}
	return append(scopes, domain.ConfigScope{
		Name:      scopeSandbox,
		Path:      filepath.Join(s.SubjectDir, filepath.FromSlash(PluginsRelDir)),
		InSandbox: true,
	})
}

// userConfigDir resolves V2's user configuration directory: XDG_CONFIG_HOME
// when set, else <home>/.config, plus the "opencode" segment.
func userConfigDir() string {
	if dir := os.Getenv(ConfigHomeEnvVar); dir != "" {
		return filepath.Join(dir, "opencode")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode")
}

// InspectScope decides whether one scope registers something that could
// rewrite the intercepted call's input. Pure over the document bytes. The
// verdict is conservative: a false "competes" is a visible refusal, a false
// "clean" would silently corrupt every verdict.
func InspectScope(scope domain.ConfigScope, doc []byte) (domain.ScopeFinding, error) {
	switch scope.Name {
	case scopeUserConfig, scopeUserConfigJSONC:
		return inspectConfigScope(scope, doc)
	case scopeUserPlugins, scopeUserPluginLegacy:
		return inspectPluginFolderScope(scope, doc), nil
	default:
		return domain.ScopeFinding{Scope: scope}, nil
	}
}

func inspectConfigScope(scope domain.ConfigScope, doc []byte) (domain.ScopeFinding, error) {
	keys, err := registeredPluginKeys(doc)
	if err != nil {
		return domain.ScopeFinding{}, fmt.Errorf("opencode-v2: parsing scope document %q: %w", scope.Path, err)
	}
	if len(keys) == 0 {
		return domain.ScopeFinding{Scope: scope}, nil
	}
	return domain.ScopeFinding{
		Scope:         scope,
		RewritesInput: true,
		Detail: fmt.Sprintf("%s declares a non-empty plugin registration (%s); any plugin present may register a hook that rewrites the intercepted call's input",
			scope.Name, quotedKeys(keys)),
	}, nil
}

func inspectPluginFolderScope(scope domain.ConfigScope, doc []byte) domain.ScopeFinding {
	if strings.TrimSpace(string(doc)) == "" {
		return domain.ScopeFinding{Scope: scope}
	}
	return domain.ScopeFinding{
		Scope:         scope,
		RewritesInput: true,
		Detail: fmt.Sprintf("%s is a non-empty plugin folder; any plugin present may register a hook that rewrites the intercepted call's input",
			scope.Name),
	}
}

// fsScopeProbe is the real filesystem ScopeProbe; it never writes.
type fsScopeProbe struct{}

func (fsScopeProbe) Read(scope domain.ConfigScope) ([]byte, error) {
	if scope.Name == scopeUserPlugins || scope.Name == scopeUserPluginLegacy {
		entries, err := os.ReadDir(scope.Path)
		if err != nil {
			return nil, probeError(scope, err)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return []byte(strings.Join(names, "\n")), nil
	}
	data, err := os.ReadFile(scope.Path)
	if err != nil {
		return nil, probeError(scope, err)
	}
	return data, nil
}

func probeError(scope domain.ConfigScope, err error) error {
	if os.IsNotExist(err) {
		return ErrScopeAbsent
	}
	return fmt.Errorf("opencode-v2: reading scope %q: %w", scope.Path, err)
}

// inspectOneScope reads and judges one non-sandbox scope, folding every
// outcome - absent, unreadable, malformed, clean, rewriting - into a finding
// that never reads as "inspected and clean" when it was not.
func inspectOneScope(probe ScopeProbe, scope domain.ConfigScope) domain.ScopeFinding {
	doc, err := probe.Read(scope)
	if err != nil {
		if errors.Is(err, ErrScopeAbsent) {
			return domain.ScopeFinding{
				Scope:                    scope,
				Neutralized:              scope.Isolatable,
				PreservesSubjectFunction: true,
				Detail:                   fmt.Sprintf("%s configuration scope not found; treated as absent, not inspected", scope.Name),
			}
		}
		return domain.ScopeFinding{
			Scope:         scope,
			RewritesInput: true,
			Detail:        fmt.Sprintf("%s configuration scope could not be read: %v; treated as unresolved for safety", scope.Name, err),
		}
	}

	finding, err := InspectScope(scope, doc)
	if err != nil {
		return domain.ScopeFinding{
			Scope:         scope,
			RewritesInput: true,
			Detail:        fmt.Sprintf("%s configuration scope document malformed: %v", scope.Name, err),
		}
	}
	finding.PreservesSubjectFunction = true
	finding.Neutralized = scope.Isolatable && !finding.RewritesInput
	return finding
}

// ConfigScopes implements domain.HarnessAdapter, independent of any sandbox.
func (a *Adapter) ConfigScopes() []domain.ConfigScope {
	return Scopes(domain.Sandbox{})
}

// InspectScopes implements domain.HarnessAdapter: it examines every
// non-sandbox scope through the adapter's probe.
func (a *Adapter) InspectScopes(_ context.Context) ([]domain.ScopeFinding, error) {
	probe := a.opts.ScopeProbe
	if probe == nil {
		probe = fsScopeProbe{}
	}
	var findings []domain.ScopeFinding
	for _, scope := range a.ConfigScopes() {
		if scope.InSandbox {
			continue
		}
		findings = append(findings, inspectOneScope(probe, scope))
	}
	return findings, nil
}
