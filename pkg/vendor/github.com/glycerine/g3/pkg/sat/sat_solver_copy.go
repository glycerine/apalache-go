package sat

// Source: src/sat/sat_solver.cpp:75-127; src/sat/sat_solver.h:257.

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (s *Solver) copyFrom(src *Solver) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if src == nil || src.self != src {
		return ErrSATOwner
	}
	if err = s.popToBase(); err != nil {
		return err
	}
	if satStorageSourceDebug {
		if !s.converter.Empty() || !src.converter.Empty() {
			return ErrSATAssertion
		}
		if s.Variables.scopeLevel != 0 {
			return ErrSATAssertion
		}
	}
	n, e := s.Variables.numVarsChecked()
	if e != nil {
		return e
	}
	sn, e := src.Variables.numVarsChecked()
	if e != nil {
		return e
	}
	if n < sn {
		v, e := s.Variables.numVarsChecked()
		if e != nil {
			return e
		}
		for ; ; v++ {
			sn, e = src.Variables.numVarsChecked()
			if e != nil {
				return e
			}
			if v >= sn {
				break
			}
			if satStorageSourceDebug {
				elim, e := src.Variables.wasEliminated(BoolVar(v))
				if e != nil {
					return e
				}
				if elim {
					return ErrSATAssertion
				}
			}
			ext, e := src.Variables.IsExternal(BoolVar(v))
			if e != nil {
				return e
			}
			dvar, e := src.Variables.decisionEligible(BoolVar(v))
			if e != nil {
				return e
			}
			created, e := s.MkVar(ext, dvar)
			if e != nil {
				return e
			}
			if satStorageSourceDebug && uint32(created) != v {
				return ErrSATAssertion
			}
		}
	}
	var sz uint32
	if src.Variables.scopeLevel == 0 {
		sz, err = src.Variables.trail.Size(src.Variables.trailRef)
	} else {
		if len(src.Lifecycle.scopes) == 0 {
			return arena.ErrBounds
		}
		sz = src.Lifecycle.scopes[0].trailLimit
	}
	if err != nil {
		return err
	}
	for i := uint32(0); i < sz; i++ {
		l, e := src.Variables.trailRead(i)
		if e != nil {
			return e
		}
		if err = s.propagation.assign(l, satJustification{}); err != nil {
			return err
		}
	}
	// The source captures outer size and each actual watch begin/end. Actual
	// arena iterator generations reject a freed captured payload at the read.
	size := len(src.Variables.watches.lists)
	for idx := 0; idx < size; idx++ {
		l := Literal(uint32(idx)).Negated()
		list, e := src.Variables.watches.Get(uint32(idx))
		if e != nil {
			return e
		}
		ref, e := list.ref()
		if e != nil {
			return e
		}
		it, e := src.Variables.watches.store.Iter(ref)
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
			// sat_watched.h:88 uses the full packed word, not only kind
			// bits plus the narrower is_learned()==1 predicate.
			if w.val2 != 0 {
				continue
			}
			l2, e := w.Literal()
			if e != nil {
				return e
			}
			if l.index() > l2.index() {
				continue
			}
			pair := [2]Literal{l, l2}
			if _, err = s.Lifecycle.MkClauseCore(2, pair[:], false); err != nil {
				return err
			}
		}
	}
	if err = s.copyOriginalClauses(src); err != nil {
		return err
	}
	if err = s.conflict.userScopeLiterals.Shrink(0); err != nil {
		return err
	}
	return satMUSAppend(&s.conflict.userScopeLiterals, &src.conflict.userScopeLiterals)
}
func (s *Solver) copyOriginalClauses(src *Solver) (err error) {
	previous := s.consequenceRunning
	s.consequenceRunning = true
	defer func() { s.consequenceRunning = previous }()
	f, e := s.consequenceFrame()
	if e != nil {
		return e
	}
	defer func() { err = s.consequenceDrop(f, err) }()
	if err = f.vector(s, 0, nil); err != nil {
		return err
	}
	buffer := &f.vectors[0]
	original := src.Lifecycle.Original
	// This concrete charged clause-vector append helper replaces backing only
	// when its geometric capacity strictly increases. The captured source end
	// remains fixed; guard relocation at each reached old-pointer dereference.
	originalCapacity := cap(original)
	for i := range original {
		if e := src.identity(); e != nil {
			return e
		}
		if cap(src.Lifecycle.Original) != originalCapacity {
			return ErrSATOwner
		}
		c := original[i]
		if err = buffer.Shrink(0); err != nil {
			return err
		}
		for i := uint32(0); ; i++ {
			n, e := src.Clauses.Size(c)
			if e != nil {
				return e
			}
			if i >= n {
				break
			}
			l, e := src.Clauses.Literal(c, i)
			if e != nil {
				return e
			}
			if err = buffer.Push(l.index()); err != nil {
				return err
			}
		}
		n, e := buffer.Size()
		if e != nil {
			return e
		}
		if _, err = s.Lifecycle.mkClauseCoreSpan(n, satConflictColumnSpan(buffer), false); err != nil {
			return err
		}
	}
	return nil
}
