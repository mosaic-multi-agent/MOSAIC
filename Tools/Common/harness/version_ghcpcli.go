package harness

import "regexp"

// ghcpCLIVersionPath matches a package path of the installed copilot CLI:
// `pkg` <sep> <os>-<arch> <sep> <version>, where <sep> is `/`, `\` or the
// JSON-escaped `\\`. `pkg` must start a path segment; the version starts with
// digits, a dot and a digit and runs over letters, digits, `.`, `-` and `+`.
var ghcpCLIVersionPath = regexp.MustCompile(
	`(?:^|[^A-Za-z0-9_-])pkg(?:\\\\|\\|/)[A-Za-z0-9_]+-[A-Za-z0-9_]+(?:\\\\|\\|/)(\d+\.\d[A-Za-z0-9.+-]*)`)

// GHCPCLIVersions returns the distinct GHCP CLI versions named by package
// paths in a raw `copilot --output-format json` stdout, in first-seen order.
// It returns nil when no path matches. It never fails and never inspects
// anything but the raw bytes.
func GHCPCLIVersions(stdout []byte) []string {
	var versions []string
	seen := map[string]bool{}
	for _, m := range ghcpCLIVersionPath.FindAllSubmatch(stdout, -1) {
		v := string(m[1])
		if !seen[v] {
			seen[v] = true
			versions = append(versions, v)
		}
	}
	return versions
}
