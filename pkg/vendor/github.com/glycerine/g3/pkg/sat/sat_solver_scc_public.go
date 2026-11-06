package sat

// SCCBinary preserves solver::scc_bin and notifies an extension only when at
// least one strongly connected component was eliminated.
func (s *Solver) SCCBinary() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if s.Variables.scopeLevel > 0 || s.propagation.inconsistent {
		return 0, nil
	}
	count, err := s.scc.run()
	if err != nil {
		return count, err
	}
	if count > 0 {
		err = s.propagation.notifyExtensionClausesModified()
	}
	return count, err
}
