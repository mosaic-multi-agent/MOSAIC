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
