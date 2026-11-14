package descriptor

import (
	"path"
	"strings"
)

// agentStem returns the file name stem to which the declared agent extension is
// appended. A name already ending in the whole declared extension (including a multi-dot
// extension such as ".agent.md") loses exactly that extension, so it is not doubled;
// any other name loses its last dotted component.
func agentStem(fileName, ext string) string {
	base := path.Base(fileName)
	if strings.HasSuffix(base, ext) {
		return strings.TrimSuffix(base, ext)
	}
	return strings.TrimSuffix(base, path.Ext(base))
}
