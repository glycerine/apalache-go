package sat

// Source: src/sat/sat_simplifier.cpp:98–122,421–458,532–681.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *satSimplifier) removeClauseCore(c StoredClauseRef) error {
	a := s.lifecycle.propagation.clauses
	n, e := a.Size(c)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		l, e := a.Literal(c, i)
		if e != nil {
			return e
		}
		if e = s.lifecycle.propagation.variables.elimTodo.Insert(uint32(l.variable())); e != nil {
			return e
		}
	}
	if e = s.subTodo.Erase(c); e != nil {
		return e
	}
	if e = a.SetRemoved(c, true); e != nil {
		return e
	}
	s.traceRemovedClause(c)
	s.needCleanup = true
	return nil
}
func (s *satSimplifier) removeClause(c StoredClauseRef) error {
	if e := s.removeClauseCore(c); e != nil {
		return e
	}
	return s.useList.Erase(c)
}
func (s *satSimplifier) removeClauseExcept(c StoredClauseRef, l Literal) error {
	if e := s.removeClauseCore(c); e != nil {
		return e
	}
	return s.useList.EraseExcept(c, l)
}
func (s *satSimplifier) cleanupClause(c StoredClauseRef, inUse bool) (bool, error) {
	a := s.lifecycle.propagation.clauses
	n, e := a.Size(c)
	if e != nil {
		return false, e
	}
	j := uint32(0)
	result := false
	for i := uint32(0); i < n; i++ {
		l, e := a.Literal(c, i)
		if e != nil {
			return false, e
		}
		v, e := s.lifecycle.propagation.variables.valueLiteral(l)
		if e != nil {
			return false, e
		}
		switch v {
		case lbool.Undef:
			if e = a.SetLiteral(c, j, l); e != nil {
				return false, e
			}
			j++
		case lbool.False:
			s.needCleanup = true
			if inUse {
				f, e := a.Frozen(c)
				if e != nil {
					return false, e
				}
				if !f {
					u, e := s.useList.Get(l)
					if e != nil {
						return false, e
					}
					if e = u.EraseNotRemoved(c); e != nil {
						return false, e
					}
				}
			}
		case lbool.True:
			result = true
			if e = a.SetLiteral(c, j, l); e != nil {
				return false, e
			}
			j++
		}
	}
	if e = a.Shrink(c, j); e != nil {
		return false, e
	}
	return result, nil
}
func (s *satSimplifier) cleanupLiterals(list satSolverLiteralListRef) (bool, error) {
	n, e := s.literalStore.Size(list)
	if e != nil {
		return false, e
	}
	j := uint32(0)
	for i := uint32(0); i < n; i++ {
		l, e := s.literalStore.Read(list, i)
		if e != nil {
			return false, e
		}
		v, e := s.lifecycle.propagation.variables.valueLiteral(l)
		if e != nil {
			return false, e
		}
		switch v {
		case lbool.Undef:
			if e = s.literalStore.Write(list, j, l); e != nil {
				return false, e
			}
			j++
		case lbool.False:
		case lbool.True:
			return true, nil
		}
	}
	return false, s.literalStore.Shrink(list, j)
}
func (s *satSimplifier) unitOccurrences(l Literal, satisfied bool) (err error) {
	target := l.Negated()
	if satisfied {
		target = l
	}
	view, e := s.useList.Get(target)
	if e != nil {
		return e
	}
	it, e := view.Begin()
	if e != nil {
		return e
	}
	// Source iterator destructor precedes cs.reset and all subsequent source reads.
	defer func() {
		if e := it.Close(); err == nil {
			err = e
		}
	}()
	for !it.AtEnd() {
		c, e := it.Current()
		if e != nil {
			return e
		}
		if e = it.Next(); e != nil {
			return e
		}
		if satisfied {
			e = s.removeClauseExcept(c, l)
		} else {
			e = s.subTodo.Insert(c)
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (s *satSimplifier) propagateUnit(l Literal) error {
	p := s.lifecycle.propagation
	v := p.variables
	old, e := v.trail.Size(v.trailRef)
	if e != nil {
		return e
	}
	if e = p.assign(l, satJustification{}); e != nil {
		return e
	}
	if _, e = p.propagateCore(false); e != nil {
		return e
	}
	if p.inconsistent {
		return nil
	}
	n, e := v.trail.Size(v.trailRef)
	if e != nil {
		return e
	}
	for i := old; i < n; i++ {
		l, e := v.trail.Read(v.trailRef, i)
		if e != nil {
			return e
		}
		if e = s.unitOccurrences(l, false); e != nil {
			return e
		}
		if e = s.unitOccurrences(l, true); e != nil {
			return e
		}
		u, e := s.useList.Get(l)
		if e != nil {
			return e
		}
		if e = u.Reset(); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSimplifier) eliminateLiteral(c StoredClauseRef, l Literal) error {
	p := s.lifecycle.propagation
	a := p.clauses
	s.traceElimLitClause(644, "processing: ", c)
	s.needCleanup = true
	s.numElimLits++
	if e := p.variables.elimTodo.Insert(uint32(l.variable())); e != nil {
		return e
	}
	if e := a.Elim(c, l); e != nil {
		return e
	}
	u, e := s.useList.Get(l)
	if e != nil {
		return e
	}
	if e = u.EraseNotRemoved(c); e != nil {
		return e
	}
	n, e := u.Size()
	if e != nil {
		return e
	}
	s.kernel.subCounter = int32(uint32(s.kernel.subCounter) - n/2)
	satisfied, e := s.cleanupClause(c, true)
	if e != nil {
		return e
	}
	if satisfied {
		s.traceElimLitText(654, "clause was satisfied\n")
		return s.removeClause(c)
	}
	n, e = a.Size(c)
	if e != nil {
		return e
	}
	switch n {
	case 0:
		s.traceElimLitText(660, "clause is empty\n")
		p.setConflict(satJustification{}, DefaultLiteral())
		return nil
	case 1:
		l, e := a.Literal(c, 0)
		if e != nil {
			return e
		}
		s.traceElimLitUnit(l)
		return s.propagateUnit(l)
	case 2:
		l1, e := a.Literal(c, 0)
		if e != nil {
			return e
		}
		l2, e := a.Literal(c, 1)
		if e != nil {
			return e
		}
		s.traceElimLitBinary(l1, l2)
		learned, e := a.Learned(c)
		if e != nil {
			return e
		}
		if e = s.lifecycle.mkBinClause(l1, l2, learned); e != nil {
			return e
		}
		// Source re-reads all three arguments after the watched-clause publication.
		l1, e = a.Literal(c, 0)
		if e != nil {
			return e
		}
		l2, e = a.Literal(c, 1)
		if e != nil {
			return e
		}
		learned, e = a.Learned(c)
		if e != nil {
			return e
		}
		if e = s.subBinTodo.Push(newSATBinClause(l1, l2, learned)); e != nil {
			return e
		}
		return s.removeClause(c)
	default:
		s.traceElimLitClause(676, "result: ", c)
		return s.subTodo.Insert(c)
	}
}
func (s *satSimplifier) resetBackward(literals bool) error {
	if e := s.bsCS.Reset(); e != nil {
		return e
	}
	if literals {
		if s.bsLS == (satSolverLiteralListRef{}) {
			var e error
			s.bsLS, e = s.literalStore.NewList()
			if e != nil {
				return e
			}
		}
		return s.literalStore.Shrink(s.bsLS, 0)
	}
	return nil
}
func (s *satSimplifier) backSubsumption1(c1 StoredClauseRef) error {
	if e := s.resetBackward(true); e != nil {
		return e
	}
	if e := s.kernel.collectSubsumed1(c1, s.bsCS, s.literalStore, s.bsLS); e != nil {
		return e
	}
	n := s.bsCS.Size()
	if satStorageSourceDebug {
		m, e := s.literalStore.Size(s.bsLS)
		if e != nil {
			return e
		}
		if n != m {
			return ErrSATAssertion
		}
	}
	for i := uint32(0); i < n; i++ {
		c2, e := s.bsCS.Get(i)
		if e != nil {
			return e
		}
		dead, e := s.lifecycle.propagation.clauses.Removed(c2)
		if e != nil {
			return e
		}
		l := DefaultLiteral()
		if !dead {
			l, e = s.literalStore.Read(s.bsLS, i)
			if e != nil {
				return e
			}
		}

		if !dead && l == DefaultLiteral() {
			a1, e := s.kernel.clauseReader(c1)
			if e != nil {
				return e
			}
			learned, e := a1.Learned(c1)
			if e != nil {
				return e
			}
			if learned {
				l2, e := s.lifecycle.propagation.clauses.Learned(c2)
				if e != nil {
					return e
				}
				if !l2 {
					if e = a1.UnsetLearned(c1); e != nil {
						return e
					}
				}
			}
			s.traceSubsumed(c1, c2)
			if e = s.removeClause(c2); e != nil {
				return e
			}
			s.numSubsumed++
		} else {
			// Source ELSE IF performs a second was_removed read, not a cached boolean.
			dead, e = s.lifecycle.propagation.clauses.Removed(c2)
			if e != nil {
				return e
			}
			if !dead {
				l, e = s.literalStore.Read(s.bsLS, i)
				if e != nil {
					return e
				}
				s.traceSubsumptionResolution(c1, c2, l)
				if e = s.eliminateLiteral(c2, l); e != nil {
					return e
				}
				s.numSubRes++
				s.traceSubsumptionResult(c2)
			}
		}
		if s.lifecycle.propagation.inconsistent {
			break
		}
	}
	return nil
}
func (s *satSimplifier) backSubsumptionBinary(l1, l2 Literal, learned bool) error {
	if e := s.dummy.SetBinary(l1, l2, learned); e != nil {
		return e
	}
	return s.backSubsumption1(s.dummy.Get())
}
func (s *satSimplifier) backSubsumption0(c1 StoredClauseRef) error {
	if e := s.resetBackward(false); e != nil {
		return e
	}
	if e := s.kernel.collectSubsumed0(c1, s.bsCS); e != nil {
		return e
	}
	n := s.bsCS.Size()
	for i := uint32(0); i < n; i++ {
		c2, e := s.bsCS.Get(i)
		if e != nil {
			return e
		}
		a1, e := s.kernel.clauseReader(c1)
		if e != nil {
			return e
		}
		learned, e := a1.Learned(c1)
		if e != nil {
			return e
		}
		if learned {
			other, e := s.lifecycle.propagation.clauses.Learned(c2)
			if e != nil {
				return e
			}
			if !other {
				if e = a1.UnsetLearned(c1); e != nil {
					return e
				}
			}
		}
		s.traceSubsumed0(c1, c2)
		if e = s.removeClause(c2); e != nil {
			return e
		}
		s.numSubsumed++
	}
	return nil
}
