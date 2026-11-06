package polynomial

// Source: src/math/polynomial/linear_eq_solver.h:31-34,87-150.

func (s *RationalLinearSolver) NumericExecuting() bool { return s != nil && s.self == s && s.running }
func (s *RationalLinearSolver) beginExecution() error {
	if e := s.integer.BindCaller(&s.integerBinding, s, 0); e != nil {
		return e
	}
	if e := s.integer.Natural.BindCaller(&s.naturalBinding, s, 0); e != nil {
		_ = s.integerBinding.Unbind()
		return e
	}
	s.running = true
	return nil
}
func (s *RationalLinearSolver) endExecution() {
	if e := s.naturalBinding.Unbind(); e != nil {
		panic(e)
	}
	if e := s.integerBinding.Unbind(); e != nil {
		panic(e)
	}
	s.running = false
}
func (s *ModularLinearSolver) NumericExecuting() bool { return s != nil && s.self == s && s.running }
func (s *ModularLinearSolver) beginExecution() error {
	if e := s.integer.BindCaller(&s.integerBinding, s, 0); e != nil {
		return e
	}
	if e := s.integer.Natural.BindCaller(&s.naturalBinding, s, 0); e != nil {
		_ = s.integerBinding.Unbind()
		return e
	}
	if e := s.manager.BindCaller(&s.modularBinding, s, 0); e != nil {
		_ = s.naturalBinding.Unbind()
		_ = s.integerBinding.Unbind()
		return e
	}
	s.running = true
	return nil
}
func (s *ModularLinearSolver) endExecution() {
	if e := s.modularBinding.Unbind(); e != nil {
		panic(e)
	}
	if e := s.naturalBinding.Unbind(); e != nil {
		panic(e)
	}
	if e := s.integerBinding.Unbind(); e != nil {
		panic(e)
	}
	s.running = false
}
