package domain

// EffectiveStub returns Stub, or Subject when Stub is empty.
func (s ModelSelection) EffectiveStub() string {
	if s.Stub == "" {
		return s.Subject
	}
	return s.Stub
}

// TierModels returns exactly {TierTestSubject: s.Subject, TierTestStub:
// s.EffectiveStub()}. It is the only tier-to-model derivation in the module.
func (s ModelSelection) TierModels() map[string]string {
	return map[string]string{
		TierTestSubject: s.Subject,
		TierTestStub:    s.EffectiveStub(),
	}
}
