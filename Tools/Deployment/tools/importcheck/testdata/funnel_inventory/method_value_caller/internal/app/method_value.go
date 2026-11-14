package app

type holder struct{ d deployedBytes }

type deployedBytes struct{}

func (deployedBytes) canonicalReader() int { return 1 }

func methodValue(h holder) func() int { return h.d.canonicalReader }
