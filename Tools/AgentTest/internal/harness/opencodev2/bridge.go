package opencodev2

// Interceptor bridge construction: one bridge per interception phase, baked
// into the generated plugin.

import (
	"fmt"

	"mosaic-agent-test/internal/domain"
)

// bridge is the interceptor invocation for one phase.
type bridge struct {
	// executable is the interceptor binary from the provision request.
	executable string

	// args are the driver's interceptor arguments followed by the control
	// directory, the harness identity and the phase, all explicit: the plugin
	// runs inside the V2 server, from wherever that process happens to be.
	args []string
}

// buildBridge composes the bridge for one phase. An empty InterceptorPath is
// refused.
func buildBridge(req domain.ProvisionRequest, phase domain.InterceptionPhase) (bridge, error) {
	if req.InterceptorPath == "" {
		return bridge{}, fmt.Errorf("opencode-v2: building bridge: ProvisionRequest.InterceptorPath is empty")
	}
	args := make([]string, 0, len(req.InterceptorArgs)+6)
	args = append(args, req.InterceptorArgs...)
	args = append(args,
		"--control-dir", req.Sandbox.ControlDir,
		"--harness", HarnessID,
		"--phase", string(phase),
	)
	return bridge{executable: req.InterceptorPath, args: args}, nil
}
