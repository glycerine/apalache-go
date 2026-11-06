package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:271-302,804-812.

func (s *unconstrainedEqualitySimplifier) eliminateFromClause(variable uint32, clause *Clause) (bool, error) {
	for _, literal := range clause.Literals() {
		atom, err := s.solver.clauses.literalAtom(literal)
		if err != nil {
			return false, err
		}
		inequality, ok := atom.(*InequalityAtom)
		if !ok || inequality == nil || inequality.Kind() != AtomEQ || literal.Sign() || inequality.Size() != 1 {
			continue
		}
		even, cause := inequality.IsEven(0)
		if cause != nil {
			return false, cause
		}
		if even {
			continue
		}
		p, cause := inequality.Polynomial(0)
		if cause != nil {
			return false, cause
		}
		core, cause := s.solver.config.Polynomial.ArithmeticCore()
		if cause != nil {
			return false, cause
		}
		degree, cause := core.Degree(p, variable)
		if cause != nil {
			return false, cause
		}
		if degree != 1 {
			continue
		}
		matched, cause := s.rememberLinearEquality(variable, p)
		if cause != nil || matched {
			return matched, cause
		}
	}
	return false, nil
}
