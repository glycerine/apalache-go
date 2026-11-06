package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:688-1037,1458-1563; AXBSC
// same-sign specialization, conservative for general polynomial products.

import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) SimpleCheckPowerSums() (consistent bool, learned []sat.Literal, err error) {
	if err = s.open(); err != nil {
		return false, nil, err
	}
	checker, err := newSimpleSumChecker(s)
	if err != nil {
		return false, nil, err
	}
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
