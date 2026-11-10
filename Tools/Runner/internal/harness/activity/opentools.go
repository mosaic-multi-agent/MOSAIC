package activity

// openTools tracks the tool calls that started and have not finished.
type openTools struct {
	calls []openTool // oldest first
}

type openTool struct{ id, name string }

func (t *openTools) start(id, name string) {
	t.calls = append(t.calls, openTool{id: id, name: name})
}

// finish closes the call with the given id; unknown ids change nothing.
func (t *openTools) finish(id string) {
	for i, c := range t.calls {
		if c.id == id {
			t.calls = append(t.calls[:i], t.calls[i+1:]...)
			return
		}
	}
}

// newest returns the most recently started open call, or empty strings.
func (t *openTools) newest() (name, id string) {
	if len(t.calls) == 0 {
		return "", ""
	}
	c := t.calls[len(t.calls)-1]
	return c.name, c.id
}
