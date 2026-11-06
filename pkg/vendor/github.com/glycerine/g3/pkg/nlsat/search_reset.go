package nlsat

// Source: src/nlsat/nlsat_solver.cpp:225.

import "github.com/glycerine/g3/pkg/sat"

func (r *searchState) resetForSolver() error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.undoUntilSize(0); err != nil {
		return err
	}
	r.bk = 0
	r.xk = NullVariable
	r.level = 0
	r.core = r.core[:0]
	r.pendingLazy = Justification{}
	if r.store.atoms.bools.count == 0 {
		r.bk = sat.NullBoolVar
	}
	return nil
}
