package opencodev2

// Provision and Deprovision. V2 loads only top-level files of
// .opencode/plugins/ as plugins, so the logger variant's entry lands at the
// top level and its helpers in a subfolder. Anything already present in either
// project plugin folder, or any plugin entry in project config, is a foreign
// rewriter and is refused before anything is written.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mosaic-agent-test/internal/domain"
)

// projectPluginFolders are the project plugin folders V2 loads, relative to
// the subject directory.
var projectPluginFolders = []string{".opencode/plugins", ".opencode/plugin"}

// projectConfigFiles are the project config files V2 reads, relative to the
// subject directory.
var projectConfigFiles = []string{"opencode.json", "opencode.jsonc", ".opencode/opencode.json", ".opencode/opencode.jsonc"}

// interceptorContribution is the generated interceptor plugin. It declares
// RewritesInput, which ComposePlugins checks against every other source.
func interceptorContribution(req domain.ProvisionRequest) (PluginContribution, bool, error) {
	phases := []domain.InterceptionPhase{domain.PhasePre, domain.PhasePost, domain.PhaseCompletion}
	bridges := make([]bridge, len(phases))
	for i, phase := range phases {
		b, err := buildBridge(req, phase)
		if err != nil {
			return PluginContribution{}, false, err
		}
		bridges[i] = b
	}
	content, err := generatePlugin(bridges[0], bridges[1], bridges[2])
	if err != nil {
		return PluginContribution{}, false, err
	}
	return PluginContribution{
		Source:        "mosaic-interceptor",
		RewritesInput: true,
		Files:         []PluginFile{{Target: PluginFileName, Content: content}},
	}, true, nil
}

// Provision implements domain.HarnessAdapter. It deploys the interceptor
// plugin slot and the logger bundle's V2 variant under req.Sandbox only, and
// refuses when a foreign plugin able to rewrite the intercepted call's input
// would be present. Nothing is written when a refusal happens before the
// writes; on a failure during the writes the returned ledger holds what was
// created so the caller can tear it down through Deprovision.
func (a *Adapter) Provision(_ context.Context, req domain.ProvisionRequest) (domain.Provisioning, error) {
	prov := domain.Provisioning{Sandbox: req.Sandbox}

	if err := checkNoForeignPlugins(req.Sandbox.SubjectDir); err != nil {
		return prov, err
	}

	var contribs []PluginContribution
	if c, ok, err := interceptorContribution(req); err != nil {
		return prov, err
	} else if ok {
		contribs = append(contribs, c)
	}
	if req.LoggerBundleDir != "" {
		c, err := loadLoggerBundleContribution(req.LoggerBundleDir)
		if err != nil {
			return prov, err
		}
		contribs = append(contribs, c)
	}

	files, err := ComposePlugins(contribs...)
	if err != nil {
		return prov, err
	}

	pluginsDir := filepath.Join(req.Sandbox.SubjectDir, filepath.FromSlash(PluginsRelDir))
	dests := make([]string, len(files))
	for i, f := range files {
		if dests[i], err = safeJoin(pluginsDir, f.Target); err != nil {
			return prov, fmt.Errorf("opencode-v2: plugin file target %q: %w", f.Target, err)
		}
	}
	for i, f := range files {
		if err := writeTracked(&prov, dests[i], f.Content); err != nil {
			return prov, err
		}
	}

	prov.ScopeFindings = relocatedScopeFindings(req.Sandbox)
	return prov, nil
}

// relocatedScopeFindings accounts for every user scope at its relocated path:
// the spawn plan points the config home into the run's control directory, so
// each run reads its own location. InSandbox is false so cross-run isolation
// checks, which skip in-sandbox scopes, still compare these paths.
func relocatedScopeFindings(sb domain.Sandbox) []domain.ScopeFinding {
	var findings []domain.ScopeFinding
	for _, s := range Scopes(sb) {
		if s.Name == scopeSandbox {
			continue
		}
		findings = append(findings, domain.ScopeFinding{
			Scope:                    domain.ConfigScope{Name: s.Name, Path: s.Path, InSandbox: false, Isolatable: true},
			Neutralized:              true,
			PreservesSubjectFunction: true,
		})
	}
	return findings
}

// checkNoForeignPlugins refuses a subject directory that already holds a
// plugin or a plugin registration: nothing has been written by this
// provisioning yet, so anything present is someone else's. Unreadable or
// malformed project config is refused as unjudgeable.
func checkNoForeignPlugins(subjectDir string) error {
	for _, rel := range projectPluginFolders {
		dir := filepath.Join(subjectDir, filepath.FromSlash(rel))
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("opencode-v2: reading plugins directory %q: %w", dir, err)
		}
		if len(entries) == 0 {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return fmt.Errorf("%w: plugins directory %q already contains %s", ErrMultipleRewriters, dir, strings.Join(names, ", "))
	}
	for _, rel := range projectConfigFiles {
		if err := checkProjectConfig(filepath.Join(subjectDir, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	return nil
}

func checkProjectConfig(path string) error {
	doc, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("opencode-v2: reading project config %q: %w", path, err)
	}
	keys, err := registeredPluginKeys(doc)
	if err != nil {
		return fmt.Errorf("%w: project config %q is malformed and cannot be judged: %v", ErrMultipleRewriters, path, err)
	}
	if len(keys) > 0 {
		return fmt.Errorf("%w: project config %q registers plugins (%s)", ErrMultipleRewriters, path, quotedKeys(keys))
	}
	return nil
}

// Deprovision implements domain.HarnessAdapter. It removes exactly what
// Provisioning records, files before directories, and is idempotent.
func (a *Adapter) Deprovision(_ context.Context, p domain.Provisioning) error {
	for i := len(p.Files) - 1; i >= 0; i-- {
		if err := os.Remove(p.Files[i]); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("opencode-v2: removing %q: %w", p.Files[i], err)
		}
	}
	for i := len(p.Dirs) - 1; i >= 0; i-- {
		if err := os.Remove(p.Dirs[i]); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("opencode-v2: removing directory %q: %w", p.Dirs[i], err)
		}
	}
	return nil
}
