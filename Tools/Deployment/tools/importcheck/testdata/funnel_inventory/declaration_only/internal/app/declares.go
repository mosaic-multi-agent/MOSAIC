package app

type holder struct{}

// canonicalReader declares a method with a funnel name; a declaration is not a reference.
func (h holder) canonicalReader() int { return 1 }

func funnelReader() int { return 2 }
