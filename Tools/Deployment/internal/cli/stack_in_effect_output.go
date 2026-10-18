package cli

// stack_in_effect_output.go reports the stack in effect (the catalog layers a run used and
// where that stack came from) in the output of every catalog-loading subcommand. Human output
// gets a leading block for the stack-input and persisted origins; JSON output gets one extra
// top-level key in the subcommand's own document, for every origin.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
)

// withRunStackInEffect is withStackInEffect for the documents that are a domain.RunSummary,
// whose keys carry Go field names.
func withRunStackInEffect(out io.Writer, svc app.Service, format string) io.Writer {
	return withStackInEffect(out, svc, format, "StackInEffect")
}

// withDocStackInEffect is withStackInEffect for the lowerCamel documents (render, promote,
// transform, check-index).
func withDocStackInEffect(out io.Writer, svc app.Service, format string) io.Writer {
	return withStackInEffect(out, svc, format, "stackInEffect")
}

// withStackInEffect returns the writer a subcommand renders its result to. In human mode it
// first writes the stack block (nothing for the default origin) and returns out itself. In
// JSON mode it returns a writer that adds the stack under key to the document written through it.
func withStackInEffect(out io.Writer, svc app.Service, format, key string) io.Writer {
	active := svc.ActiveStack()
	if !strings.EqualFold(format, "json") {
		writeStackBlock(out, active)
		return out
	}
	return &stackKeyWriter{out: out, key: key, active: active}
}

// writeStackBlock writes the "Catalog stack" block, one line per layer, highest first. The
// default origin writes nothing, so default-only runs keep their output.
func writeStackBlock(out io.Writer, active stackconfig.ActiveStack) {
	if active.Origin != stackconfig.OriginStackInput && active.Origin != stackconfig.OriginPersisted {
		return
	}
	fmt.Fprintf(out, "Catalog stack (%s):\n", active.Origin)
	for i, l := range active.Layers {
		marker := ""
		if l.Default {
			marker = " (default)"
		}
		fmt.Fprintf(out, "  %d. %s %s %s %s%s\n", i+1, l.Identity.ID, l.Identity.Name, l.Identity.Version, l.Root, marker)
	}
}

// stackKeyWriter adds the stack in effect to a JSON document as one extra top-level key. A
// subcommand encodes its document in a single Write; the existing keys keep their order and
// content, and the new key goes last.
type stackKeyWriter struct {
	out    io.Writer
	key    string
	active stackconfig.ActiveStack
}

func (w *stackKeyWriter) Write(p []byte) (int, error) {
	doc := bytes.TrimRight(p, "\n")
	if len(doc) < 2 || doc[0] != '{' || doc[len(doc)-1] != '}' {
		return w.out.Write(p)
	}
	value, err := json.Marshal(w.active)
	if err != nil {
		return w.out.Write(p)
	}
	body := bytes.TrimRight(doc[1:len(doc)-1], " \n\t")
	var merged bytes.Buffer
	merged.WriteByte('{')
	merged.Write(body)
	if len(body) > 0 {
		merged.WriteByte(',')
	}
	fmt.Fprintf(&merged, "%q:", w.key)
	merged.Write(value)
	merged.WriteString("}\n")
	if _, err := w.out.Write(merged.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}
