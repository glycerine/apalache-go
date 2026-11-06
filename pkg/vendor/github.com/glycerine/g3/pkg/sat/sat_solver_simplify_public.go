package sat

// SimplifyClauses preserves solver::simplify, including finalize before the
// extension notification.
func (s *Solver) SimplifyClauses(learned bool) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.Variables.scopeLevel > 0 || s.propagation.inconsistent {
		return nil
	}
	if err := s.simplifier.Run(learned); err != nil {
		return err
	}
	if err := s.simplifier.Finalize(); err != nil {
		return err
	}
	return s.propagation.notifyExtensionClausesModified()
}
