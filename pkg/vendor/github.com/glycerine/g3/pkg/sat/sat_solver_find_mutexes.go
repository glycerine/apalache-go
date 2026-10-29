package sat

// Source: src/sat/sat_solver.cpp:2818-2838,3119-3150; src/util/max_cliques.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/util/cliques"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type satMutexFrame struct {
	next       *satMutexFrame
	ownerBytes uint64
	seen       map[[2]uint32]struct{}
}

func (s *Solver) mutexFrame() (*satMutexFrame, error) {
	bytes := uint64(reflect.TypeOf(satMutexFrame{}).Size())
	if e := s.budget.Reserve(bytes); e != nil {
		return nil, e
	}
	f := &satMutexFrame{next: s.mutexFrames, ownerBytes: bytes, seen: make(map[[2]uint32]struct{})}
	s.mutexFrames = f
	return f, nil
}
func (s *Solver) mutexDrop(f *satMutexFrame, cause error) error {
	if s.mutexFrames != f {
		if cause != nil {
			return cause
		}
		return ErrSATOwner
	}
	f.seen = nil
	s.mutexFrames = f.next
	s.budget.Release(f.ownerBytes)
	f.ownerBytes = 0
	return cause
}
func (s *Solver) cleanupMutexes() error {
	for s.mutexFrames != nil {
		if e := s.mutexDrop(s.mutexFrames, nil); e != nil {
			return e
		}
	}
	return nil
}
func (s *Solver) collectMutexBins(learned bool) error {
	for idx := 0; idx < len(s.Variables.watches.lists); idx++ {
		l := Literal(uint32(idx)).Negated()
		it, e := s.Variables.watches.store.Iter(s.Variables.watches.lists[idx])
		if e != nil {
			return e
		}
		for {
			w, yes, e := it.Next()
			if e != nil {
				return e
			}
			if !yes {
				break
			}
			if !w.Binary() {
				continue
			}
			if !learned {
				yes, e := w.Learned()
				if e != nil {
					return e
				}
				if yes {
					continue
				}
			}
			l2, e := w.Literal()
			if e != nil {
				return e
			}
			if l.index() > l2.index() {
				continue
			}
			s.userBinClauses = append(s.userBinClauses, newSATSolverBinPair(l, l2))
		}
	}
	return nil
}
func (s *Solver) findMutexes(lits []Literal, out *satSolverLiteralRows) (result lbool.LBool, err error) {
	if err = s.consequenceEntry(out); err != nil {
		return lbool.Undef, err
	}
	s.mutexRunning = true
	defer func() { s.mutexRunning = false }()
	f, e := s.mutexFrame()
	if e != nil {
		return lbool.Undef, e
	}
	defer func() { err = s.mutexDrop(f, err) }()
	graph := cliques.New()
	s.userBinClauses = s.userBinClauses[:0]
	// Existing concrete empty/member graph reset. Actual owning rows destroy in
	// ascending order; retained outer cold backing is not a source malloc ticket.
	for i := range s.binaryClauseGraph {
		if err = s.binaryClauseGraph[i].Finalize(); err != nil {
			return lbool.Undef, err
		}
	}
	s.binaryClauseGraph = s.binaryClauseGraph[:0]
	if err = s.collectMutexBins(true); err != nil {
		return lbool.Undef, err
	}
	if err = s.collectMutexBins(false); err != nil {
		return lbool.Undef, err
	}
	for _, bc := range s.userBinClauses {
		a, b := bc.Literal1().index(), bc.Literal2().index()
		key := [2]uint32{a, b}
		if _, seen := f.seen[key]; !seen {
			f.seen[key] = struct{}{}
			if err = graph.AddEdge(a, b); err != nil {
				return lbool.Undef, err
			}
		}
	}
	ps := make([]uint32, len(lits))
	for i, l := range lits {
		ps[i] = l.index()
	}
	results, e := graph.Cliques(ps)
	if e != nil {
		return lbool.Undef, e
	}
	for _, row := range results {
		if err = out.appendUnsigned(row); err != nil {
			return lbool.Undef, err
		}
	}
	return lbool.True, nil
}
