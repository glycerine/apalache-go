package nlsat

// Source: src/nlsat/nlsat_solver.cpp:578-582; nlsat_simplify.cpp:75-80.

import "github.com/glycerine/g3/pkg/math/polynomial"

func (s *Solver) polynomialVariablesAreInteger(p polynomial.Polynomial) (bool, error) {
	core := s.atoms.ineq.core
	info, err := core.Info(p)
	if err != nil {
		return false, err
	}
	mm, err := core.MonomialManager()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < info.Size; i++ {
		term, cause := core.TermMonomial(p, i)
		if cause != nil {
			return false, cause
		}
		termInfo, cause := mm.Info(term)
		if cause != nil {
			return false, cause
		}
		for j := uint32(0); j < termInfo.Size; j++ {
			power, cause := mm.ReadPower(term, j)
			if cause != nil {
				return false, cause
			}
			integer, cause := s.IsInteger(power.Var)
			if cause != nil || !integer {
				return false, cause
			}
		}
	}
	return true, nil
}
