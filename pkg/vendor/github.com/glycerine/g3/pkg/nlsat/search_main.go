package nlsat

// Source: src/nlsat/nlsat_solver.cpp:1204-1256,1342-1352.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (r *searchState) initializeSearch() error {
	if err := r.undoUntilEmpty(); err != nil {
		return err
	}
	r.xk = NullVariable
	r.bk = 0
	for i := range r.store.atoms.bools.values {
		r.store.atoms.bools.values[i] = lbool.Undef
	}
	return r.assignment.Reset()
}

// search executes the retained Boolean/arithmetic stage loop. Integer branch
// and bound is the separate source search_check layer and remains outside it.
func (r *searchState) search(resolution *searchResolution, explanation *explanationAssumptions) (lbool.LBool, error) {
	if resolution == nil || resolution.search != r {
		return lbool.Undef, ErrSearchOwner
	}
	if explanation != nil && explanation.search != r {
		return lbool.Undef, ErrSearchOwner
	}
	if err := r.initializeSearch(); err != nil {
		return lbool.Undef, err
	}

	for {
		if r.xk == NullVariable {
			if err := r.peekNextBooleanVariable(); err != nil {
				return lbool.Undef, err
			}
			if r.bk == sat.NullBoolVar {
				if err := r.newStage(); err != nil {
					return lbool.Undef, err
				}
			}
		} else if err := r.newStage(); err != nil {
			return lbool.Undef, err
		}

		if r.bk == sat.NullBoolVar && uint64(r.xk) >= uint64(len(r.store.vars.isInt)) {
			return lbool.True, nil
		}

		for {
			if err := r.checkpoint(); err != nil {
				return lbool.Undef, err
			}
			var watches []*Clause
			if r.xk == NullVariable {
				if uint64(r.bk) >= uint64(len(r.store.atoms.bools.watches)) {
					return lbool.Undef, arena.ErrBounds
				}
				watches = r.store.atoms.bools.watches[r.bk]
			} else {
				if uint64(r.xk) >= uint64(len(r.store.vars.watches)) {
					return lbool.Undef, arena.ErrBounds
				}
				watches = r.store.vars.watches[r.xk]
			}
			conflict, err := r.processClauses(watches)
			if err != nil {
				return lbool.Undef, err
			}
			if conflict == nil {
				break
			}
			solved, err := resolution.resolveConflict(conflict, explanation)
			if err != nil {
				return lbool.Undef, err
			}
			if !solved {
				return lbool.False, nil
			}
			if r.conflicts >= r.config.MaxConflicts {
				return lbool.Undef, nil
			}
		}

		if r.xk == NullVariable {
			if uint64(r.bk) >= uint64(len(r.store.atoms.bools.values)) {
				return lbool.Undef, arena.ErrBounds
			}
			if r.store.atoms.bools.values[r.bk] == lbool.Undef {
				if err := r.decide(sat.MakeLiteral(r.bk, true)); err != nil {
					return lbool.Undef, err
				}
				r.bk++
			}
		} else if err := r.selectWitness(); err != nil {
			return lbool.Undef, err
		}
	}
}
