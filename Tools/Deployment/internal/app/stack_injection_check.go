package app

// CheckHarnessInjections reports whether the active stack supplies the injections of the harness
// with the given id. It returns the error the registry raises for the harness's injection item
// (the same named error the no-fallback rule raises at deploy time), or nil when supplied.
func (s *service) CheckHarnessInjections(harnessID string) error {
	_, err := s.deps.Registry.Resolve(harnessID)
	return err
}
