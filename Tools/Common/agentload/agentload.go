// Package agentload loads one deployed agent file, read-only, in any registered
// agent format.
package agentload

import (
	"errors"
	"fmt"
	"os"

	"mosaic-common/docformat"

	"mosaic-common/agentformat"
	_ "mosaic-common/agentformat/all"
	"mosaic-common/agentformat/formatid"
)

// Setting is one native setting of an agent file (alias, so callers need not import
// agentformat).
type Setting = agentformat.Setting

// Agent is the read-only result of loading one agent file.
type Agent struct {
	Path      string              // the path given to Load
	FormatID  string              // resolved format id ("markdown" when "" was given)
	Canonical []byte              // standard form bytes (MOSAIC Markdown document)
	Document  *docformat.Document // parsed Canonical
	Settings  []Setting           // nil unless the translator implements SettingsReader
}

// LoadError is the failure of loading one agent file.
type LoadError struct {
	Path     string
	FormatID string
	// Err wraps exactly one of ErrUnknownFormat, ErrUnreadable, ErrUndecodable, plus
	// the cause.
	Err error
}

// Error returns "agent file <Path> (<FormatID>): <reason>: <cause>".
func (e *LoadError) Error() string {
	return "agent file " + e.Path + " (" + e.FormatID + "): " + e.Err.Error()
}

// Unwrap returns the wrapped error.
func (e *LoadError) Unwrap() error { return e.Err }

var (
	// ErrUnknownFormat reports a formatID that resolves to no translator.
	ErrUnknownFormat = agentformat.ErrUnknownFormat
	// ErrUnreadable reports a file that cannot be read.
	ErrUnreadable = errors.New("agentload: agent file cannot be read")
	// ErrUndecodable reports a file that cannot be decoded.
	ErrUndecodable = errors.New("agentload: agent file cannot be decoded")
)

// Load reads the file at path and decodes it with the translator registered for
// formatID ("" means Markdown; test-only formats resolve when registered through
// agentformat.RegisterTestOnly). It never writes to path.
//
// Settings are filled when the format's translator implements
// agentformat.SettingsReader. A failure is a *LoadError wrapping exactly one of
// ErrUnknownFormat, ErrUnreadable, ErrUndecodable.
func Load(path, formatID string) (*Agent, error) {
	resolved := formatID
	if resolved == "" {
		resolved = string(formatid.Markdown)
	}
	fail := func(sentinel, cause error) (*Agent, error) {
		return nil, &LoadError{
			Path:     path,
			FormatID: resolved,
			Err:      fmt.Errorf("%w: %w", sentinel, cause),
		}
	}

	translator, err := agentformat.LookupString(formatID)
	if err != nil {
		if errors.Is(err, ErrUnknownFormat) {
			return nil, &LoadError{Path: path, FormatID: resolved, Err: err}
		}
		return fail(ErrUnknownFormat, err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fail(ErrUnreadable, err)
	}

	canonical, err := agentformat.DecodeAgentFile(translator, raw)
	if err != nil {
		return fail(ErrUndecodable, err)
	}
	doc, err := docformat.Parse(canonical)
	if err != nil {
		return fail(ErrUndecodable, err)
	}

	agent := &Agent{
		Path:      path,
		FormatID:  resolved,
		Canonical: canonical,
		Document:  doc,
	}
	if reader, ok := translator.(agentformat.SettingsReader); ok {
		settings, err := reader.ReadSettings(raw)
		if err != nil {
			return fail(ErrUndecodable, err)
		}
		agent.Settings = settings
	}
	return agent, nil
}
