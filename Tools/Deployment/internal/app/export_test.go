package app

// export_test.go exposes test-only constructors of this package to the external test package.
// It is compiled only into the tests, so production code cannot override target detection.

// ghcpTargetsOverrider is implemented by the production service to let the test-only
// constructor fix the GHCP targets that its detection reports for every workspace. Both the
// deploy-flow refusal and the pair Transform both-present check must obtain their targets
// through the service, so that one override reaches both.
type ghcpTargetsOverrider interface {
	overrideGhcpTargets(held ghcpTargets)
}

// openCodeTargetsOverrider is implemented by the production service to let the test-only
// constructor fix the OpenCode targets that its detection reports for every workspace. Both the
// deploy-flow refusal and the pair Transform both-present check must obtain their targets
// through the service, so that one override reaches both. It is independent of the GHCP hook.
type openCodeTargetsOverrider interface {
	overrideOpenCodeTargets(v1, v2 bool)
}

// NewWithOpenCodeTargetsHeld returns a Service built exactly like New(d), except that its
// OpenCode target detection reports {opencode: v1, opencode-v2: v2} for every workspace,
// whatever the manifest holds. It panics when the service lacks the override hook, so a missing
// seam never passes silently.
func NewWithOpenCodeTargetsHeld(d Deps, v1, v2 bool) Service {
	svc := New(d)
	o, ok := svc.(openCodeTargetsOverrider)
	if !ok {
		panic("app.NewWithOpenCodeTargetsHeld: service has no overrideOpenCodeTargets hook; " +
			"deploy-flow refusal and pair Transform must read targets from the service")
	}
	o.overrideOpenCodeTargets(v1, v2)
	return svc
}

// NewWithGhcpTargetsHeld returns a Service built exactly like New(d), except that its GHCP
// target detection reports {CLI: cli, VSCode: vscode} for every workspace, whatever the
// manifest holds. It is the only way to reach the "both targets present" refusal paths.
// It panics when the service lacks the override hook, so a missing seam never passes silently.
func NewWithGhcpTargetsHeld(d Deps, cli, vscode bool) Service {
	svc := New(d)
	o, ok := svc.(ghcpTargetsOverrider)
	if !ok {
		panic("app.NewWithGhcpTargetsHeld: service has no overrideGhcpTargets hook; " +
			"deploy-flow refusal and pair Transform must read targets from the service")
	}
	o.overrideGhcpTargets(ghcpTargets{CLI: cli, VSCode: vscode})
	return svc
}
