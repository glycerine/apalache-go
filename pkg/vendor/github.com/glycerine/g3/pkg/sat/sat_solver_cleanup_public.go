package sat

// CleanupClauses preserves solver::cleanup. The cleaner publishes an extension
// notification only when it changed the clause database.
func (s *Solver) CleanupClauses() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.cleaner.cleanup()
}
