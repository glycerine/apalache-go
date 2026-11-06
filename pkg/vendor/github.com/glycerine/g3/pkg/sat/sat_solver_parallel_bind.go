package sat

// SetParallelExchange preserves solver::set_par. The exchange is borrowed and
// must outlive every solver that remains bound to it.
func (s *Solver) SetParallelExchange(exchange *ParallelUnitExchange) error {
	if err := s.open(); err != nil {
		return err
	}
	if exchange == nil {
		s.conflict.parallel = nil
		s.conflict.parallelPresent = false
		s.conflict.parallelNumVars = 0
		s.conflict.parallelLimitIn = 0
		s.conflict.parallelLimitOut = 0
		return nil
	}
	exchange.mu.Lock()
	err := exchange.openLocked()
	exchange.mu.Unlock()
	if err != nil {
		return err
	}
	variables, err := s.Variables.numVarsChecked()
	if err != nil {
		return err
	}
	s.conflict.parallel = exchange
	s.conflict.parallelPresent = true
	s.conflict.parallelNumVars = variables
	s.conflict.parallelLimitIn = 0
	s.conflict.parallelLimitOut = 0
	return nil
}
