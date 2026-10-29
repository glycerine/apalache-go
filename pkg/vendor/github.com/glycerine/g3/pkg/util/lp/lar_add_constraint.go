package lp

// Source: src/util/lp/lar_solver.h:1031-1052; src/test/lp.cpp:2711-2807.

import "github.com/glycerine/g3/pkg/util/numeral"

// AddConstraint preserves the executable source entry: its normalization body
// is commented out, leaving a debug assertion and a native zero return. It
// neither reads the supplied expression nor installs a constraint. Live SMT
// consumers install bounds with AddVarBound instead; upstream file/example
// drivers still call this entry.
func (s *LARSolver) AddConstraint(_ []RationalSparseEntry, _ LConstraintKind, _ *numeral.Rational) (ci uint32, err error) {
	if err = s.ready(); err != nil {
		return 0, err
	}
	err = sourceAssertionFailure()
	if err != nil {
		s.stopped = true
	}
	return 0, err
}
