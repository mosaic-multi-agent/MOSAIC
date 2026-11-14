package agentformat

// Traits are the facts about an agent format that the application layer may branch
// on. The zero value is the Markdown behaviour.
type Traits struct {
	// StampsOutsideData: the format stores the provenance stamps (including the id
	// stamp) outside its data model (Codex: TOML comments, which other tools may
	// drop). A valid file of the format may therefore lack the stamps without being
	// foreign or corrupt.
	StampsOutsideData bool

	// ClosedVocabulary: the format natively holds only its own keys; every other
	// frontmatter key travels in the carriage container. Native keys of such a
	// format are harness artefacts that a rebuild into another harness strips and
	// reports.
	ClosedVocabulary bool
}

// TraitsProvider is implemented by a Translator whose format differs from the
// Markdown defaults. A translator that does not implement it has the zero Traits.
type TraitsProvider interface {
	Traits() Traits
}

// TraitsOf resolves formatID exactly as LookupString does ("" means Markdown) and
// returns the translator's traits: Traits() when it implements TraitsProvider, the
// zero Traits otherwise. An unresolvable id returns the zero Traits and the
// LookupString error.
func TraitsOf(formatID string) (Traits, error) {
	t, err := LookupString(formatID)
	if err != nil {
		return Traits{}, err
	}
	if p, ok := t.(TraitsProvider); ok {
		return p.Traits(), nil
	}
	return Traits{}, nil
}
