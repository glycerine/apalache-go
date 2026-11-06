package sat

// AsymmetricBranching preserves solver::asymmetric_branching. The source
// notifies a bound extension after every admitted run.
func (s *Solver) AsymmetricBranching() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.Variables.scopeLevel > 0 || s.propagation.inconsistent {
		return nil
	}
	if err := s.asymm.run(false); err != nil {
		return err
	}
	return s.propagation.notifyExtensionClausesModified()
}
