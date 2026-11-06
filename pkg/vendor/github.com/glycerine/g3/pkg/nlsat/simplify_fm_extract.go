package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:497-638.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (s *Solver) applyFourierMotzkin(x uint32, clauses []*Clause) (changed bool, err error) {
	var lo, hi []*fourierMotzkinBound
	defer func() {
		if changed {
			return
		}
		for _, bounds := range [][]*fourierMotzkinBound{lo, hi} {
			for i := len(bounds) - 1; i >= 0; i-- {
				err = errors.Join(err, bounds[i].close())
			}
		}
	}()
	for _, clause := range clauses {
		lower, upper, ok, cause := s.extractFourierMotzkinEquality(x, clause)
		if cause != nil {
			return false, cause
		}
		if !ok {
			continue
		}
		lo, hi = append(lo, lower), append(hi, upper)
		if err = s.substituteFourierMotzkinEquality(clauses, lower, upper); err != nil {
			return false, err
		}
		s.fmBounds = append(s.fmBounds, lower, upper)
		s.search.simplifications++
		return true, nil
	}
	allSolved := true
	for _, clause := range clauses {
		if s.clauses.owned[clause] == nil {
			continue
		}
		bound, positive, ok, cause := s.extractFourierMotzkinBound(x, clause)
		if cause != nil {
			return false, cause
		}
		if !ok {
			allSolved = false
			continue
		}
		target := &lo
		if positive {
			target = &hi
		}
		duplicate, cause := sameFourierMotzkinBound(*target, bound, s.atoms.ineq.core)
		if cause != nil {
			_ = bound.close()
			return false, cause
		}
		if duplicate {
			if cause = bound.close(); cause != nil {
				return false, cause
			}
			continue
		}
		*target = append(*target, bound)
	}
	if len(lo) == 0 && len(hi) == 0 {
		return false, nil
	}
	if lower, upper, found, cause := s.matchFourierMotzkinEquality(lo, hi); cause != nil {
		return false, cause
	} else if found {
		if err = s.substituteFourierMotzkinEquality(clauses, lower, upper); err != nil {
			return false, err
		}
		s.fmBounds = append(s.fmBounds, lower, upper)
		changed = true
		for _, bounds := range [][]*fourierMotzkinBound{lo, hi} {
			for _, bound := range bounds {
				if bound != lower && bound != upper {
					if err = bound.close(); err != nil {
						return changed, err
					}
				}
			}
		}
		s.search.simplifications += 2
		return true, nil
	}
	if !allSolved {
		return false, nil
	}
	if len(lo) >= 2 && len(hi) >= 2 && (len(lo) > 2 || len(hi) > 2) {
		return false, nil
	}
	if err = s.resolveFourierMotzkin(clauses, lo, hi); err != nil {
		return false, err
	}
	s.fmBounds = append(s.fmBounds, lo...)
	s.fmBounds = append(s.fmBounds, hi...)
	return true, nil
}

func sameFourierMotzkinBound(bounds []*fourierMotzkinBound, candidate *fourierMotzkinBound, core *polynomial.MultivariatePolynomialCore) (bool, error) {
	ca, cb, err := candidate.values()
	if err != nil {
		return false, err
	}
	for _, bound := range bounds {
		if bound.strict != candidate.strict {
			continue
		}
		a, b, cause := bound.values()
		if cause != nil {
			return false, cause
		}
		equalA, cause := core.Equal(a, ca)
		if cause != nil {
			return false, cause
		}
		equalB, cause := core.Equal(b, cb)
		if cause != nil {
			return false, cause
		}
		if equalA && equalB {
			return true, nil
		}
	}
	return false, nil
}
