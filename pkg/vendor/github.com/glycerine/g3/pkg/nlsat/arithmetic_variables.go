package nlsat

// Source: src/nlsat/nlsat_solver.cpp:99-107,411-425.

import "github.com/glycerine/g3/pkg/arena"

type arithmeticVariables struct {
	isInt      []bool
	watches    [][]*Clause
	infeasible []*IntervalSet
	equalities []ArithmeticAtom
	perm, inv  []uint32
}

func (s *clauseStore) mkVar(isInt bool) (uint32, error) {
	if err := s.open(); err != nil {
		return NullVariable, err
	}
	s.running = true
	defer func() { s.running = false }()
	x, err := s.atoms.ineq.core.MkVar()
	if err != nil {
		return NullVariable, err
	}
	v := &s.vars
	if err := atomAssert("nlsat_solver.cpp", 413, "x == num_vars()", uint64(x) == uint64(len(v.isInt))); err != nil {
		return NullVariable, err
	}
	if uint64(x) != uint64(len(v.isInt)) {
		return NullVariable, arena.ErrConfig
	}
	v.isInt = append(v.isInt, isInt)
	v.watches = append(v.watches, nil)
	v.infeasible = append(v.infeasible, nil)
	v.equalities = append(v.equalities, nil)
	v.perm = append(v.perm, x)
	v.inv = append(v.inv, x)
	for _, check := range []struct {
		line int32
		text string
		size int
	}{{420, "m_is_int.size() == m_watches.size()", len(v.watches)},
		{421, "m_is_int.size() == m_infeasible.size()", len(v.infeasible)},
		{422, "m_is_int.size() == m_var2eq.size()", len(v.equalities)},
		{423, "m_is_int.size() == m_perm.size()", len(v.perm)},
		{424, "m_is_int.size() == m_inv_perm.size()", len(v.inv)}} {
		if err := atomAssert("nlsat_solver.cpp", check.line, check.text, len(v.isInt) == check.size); err != nil {
			return NullVariable, err
		}
	}
	return x, nil
}
