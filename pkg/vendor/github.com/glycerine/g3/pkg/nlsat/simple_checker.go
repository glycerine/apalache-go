package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1529-1563, restricted to the
// complete degree-one AXBC specialization; unsupported forms stay possible.

import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) SimpleCheckLinearBounds() (consistent bool, learned []sat.Literal, err error) {
	if err = s.open(); err != nil {
		return false, nil, err
	}
	checker := newSimpleLinearChecker(s)
	defer func() { err = errors.Join(err, checker.close()) }()
	for {
		checker.improved = false
		for _, clause := range s.clauses.originals {
			ok, cause := checker.checkClause(clause)
			if cause != nil {
				return false, nil, cause
			}
			if !ok {
				return false, append([]sat.Literal(nil), checker.learned...), nil
			}
		}
		if !checker.improved {
			return true, append([]sat.Literal(nil), checker.learned...), nil
		}
	}
}
