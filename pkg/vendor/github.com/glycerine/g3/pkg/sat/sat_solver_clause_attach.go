package sat

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *satSolverClauseLifecycle) propagateBinClause(a, b Literal) (bool, error) {
	p := s.propagation
	v, e := p.variables.valueLiteral(b)
	if e != nil {
		return false, e
	}
	if v == lbool.False {
		p.binaryPropagations++
		return true, p.assign(a, newSATBinaryJustification(b))
	}
	v, e = p.variables.valueLiteral(a)
	if e != nil {
		return false, e
	}
	if v == lbool.False {
		p.binaryPropagations++
		return true, p.assign(b, newSATBinaryJustification(a))
	}
	return false, nil
}
func (s *satSolverClauseLifecycle) mkBinClause(a, b Literal, learned bool) error {
	if e := s.open(); e != nil {
		return e
	}
	reinit, e := s.propagateBinClause(a, b)
	if e != nil {
		return e
	}
	if reinit {
		if s.propagation.variables.scopeLevel == 0 {
			return nil
		}
		if !learned {
			if e = s.pushWrapper(newSATBinaryWrapper(a, b)); e != nil {
				return e
			}
		}
	}
	s.mkBinary++
	w, e := newSATBinaryWatch(b, learned)
	if e != nil {
		return e
	}
	if e = s.pushWatch(a.Negated(), w); e != nil {
		return e
	}
	w, e = newSATBinaryWatch(a, learned)
	if e != nil {
		return e
	}
	return s.pushWatch(b.Negated(), w)
}
func (s *satSolverClauseLifecycle) pushWatch(l Literal, w satWatched) error {
	list, e := s.propagation.variables.watches.Get(l.index())
	if e != nil {
		return e
	}
	return list.Push(w)
}
func (s *satSolverClauseLifecycle) pushReinitStack(c StoredClauseRef) error {
	s.traceReinitClause(c)
	if e := s.pushWrapper(newSATClauseWrapper(c)); e != nil {
		return e
	}
	return s.propagation.clauses.SetReinitStack(c, true)
}
func (s *satSolverClauseLifecycle) attachTerClause(c StoredClauseRef) (bool, error) {
	a := s.propagation.clauses
	p := s.propagation
	for i := uint32(0); i < 3; i++ {
		l, e := a.Literal(c, i)
		if e != nil {
			return false, e
		}
		x, e := a.Literal(c, (i+1)%3)
		if e != nil {
			return false, e
		}
		y, e := a.Literal(c, (i+2)%3)
		if e != nil {
			return false, e
		}
		if i == 1 {
			x, y = y, x
		}
		w, e := newSATTernaryWatch(x, y)
		if e != nil {
			return false, e
		}
		if e = s.pushWatch(l.Negated(), w); e != nil {
			return false, e
		}
	}
	if p.variables.scopeLevel > 0 {
		for _, i := range []uint32{0, 1, 2} {
			j, k := uint32(1), uint32(2)
			if i == 1 {
				j, k = 0, 2
			} else if i == 2 {
				j, k = 0, 1
			}
			x, e := a.Literal(c, j)
			if e != nil {
				return false, e
			}
			v, e := p.variables.valueLiteral(x)
			if e != nil {
				return false, e
			}
			if v != lbool.False {
				continue
			}
			y, e := a.Literal(c, k)
			if e != nil {
				return false, e
			}
			v, e = p.variables.valueLiteral(y)
			if e != nil {
				return false, e
			}
			if v != lbool.False {
				continue
			}
			l, e := a.Literal(c, i)
			if e != nil {
				return false, e
			}
			p.ternaryPropagations++
			return true, p.assign(l, newSATTernaryJustification(x, y))
		}
	}
	return false, nil
}
func (s *satSolverClauseLifecycle) swapLiteral(c StoredClauseRef, i, j uint32) error {
	a := s.propagation.clauses
	x, e := a.Literal(c, i)
	if e != nil {
		return e
	}
	y, e := a.Literal(c, j)
	if e != nil {
		return e
	}
	if e = a.SetLiteral(c, i, y); e != nil {
		return e
	}
	return a.SetLiteral(c, j, x)
}
func (s *satSolverClauseLifecycle) attachNaryClause(c StoredClauseRef) (bool, error) {
	p := s.propagation
	a := p.clauses
	off, e := a.Offset(c)
	if e != nil {
		return false, e
	}
	reinit := false
	if p.variables.scopeLevel > 0 {
		learned, e := a.Learned(c)
		if e != nil {
			return false, e
		}
		view := p.variables.clauseState()
		view.traceTags = s.traceTags
		if learned {
			i, e := view.selectLearnedWatchLiteral(a, c)
			if e != nil {
				return false, e
			}
			if e = s.swapLiteral(c, 1, i); e != nil {
				return false, e
			}
		} else {
			i, e := view.selectWatchLiteral(a, c, 0)
			if e != nil {
				return false, e
			}
			if e = s.swapLiteral(c, 0, i); e != nil {
				return false, e
			}
			i, e = view.selectWatchLiteral(a, c, 1)
			if e != nil {
				return false, e
			}
			if e = s.swapLiteral(c, 1, i); e != nil {
				return false, e
			}
		}
		l, e := a.Literal(c, 0)
		if e != nil {
			return false, e
		}
		v, e := p.variables.valueLiteral(l)
		if e != nil {
			return false, e
		}
		if v == lbool.False {
			p.clausePropagations++
			l, e = a.Literal(c, 1)
			if e != nil {
				return false, e
			}
			if e = p.assign(l, newSATClauseJustification(off)); e != nil {
				return false, e
			}
			reinit = true
		} else {
			l, e = a.Literal(c, 1)
			if e != nil {
				return false, e
			}
			v, e = p.variables.valueLiteral(l)
			if e != nil {
				return false, e
			}
			if v == lbool.False {
				p.clausePropagations++
				l, e = a.Literal(c, 0)
				if e != nil {
					return false, e
				}
				if e = p.assign(l, newSATClauseJustification(off)); e != nil {
					return false, e
				}
				reinit = true
			}
		}
	}
	n, e := a.Size(c)
	if e != nil {
		return false, e
	}
	block, e := a.Literal(c, n>>1)
	if e != nil {
		return false, e
	}
	w, e := newSATClauseWatch(block, off)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < 2; i++ {
		l, e := a.Literal(c, i)
		if e != nil {
			return false, e
		}
		if e = s.pushWatch(l.Negated(), w); e != nil {
			return false, e
		}
	}
	return reinit, nil
}
func (s *satSolverClauseLifecycle) attach(c StoredClauseRef) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	n, e := s.propagation.clauses.Size(c)
	if e != nil {
		return false, e
	}
	if satStorageSourceDebug && n <= 2 {
		return false, ErrSATAssertion
	}
	if n == 3 {
		return s.attachTerClause(c)
	}
	return s.attachNaryClause(c)
}
func (s *satSolverClauseLifecycle) attachClause(c StoredClauseRef) error {
	_, e := s.attach(c)
	return e
}
func (s *satSolverClauseLifecycle) detachBinClause(a, b Literal, learned bool) error {
	if e := s.open(); e != nil {
		return e
	}
	w, e := newSATBinaryWatch(b, learned)
	if e != nil {
		return e
	}
	l, e := s.propagation.variables.watches.Get(a.Negated().index())
	if e != nil {
		return e
	}
	r, e := l.ref()
	if e != nil {
		return e
	}
	if _, e = l.owner.store.Erase(r, w); e != nil {
		return e
	}
	w, e = newSATBinaryWatch(a, learned)
	if e != nil {
		return e
	}
	l, e = s.propagation.variables.watches.Get(b.Negated().index())
	if e != nil {
		return e
	}
	r, e = l.ref()
	if e != nil {
		return e
	}
	_, e = l.owner.store.Erase(r, w)
	return e
}
func (s *satSolverClauseLifecycle) detachClause(c StoredClauseRef) error {
	if e := s.open(); e != nil {
		return e
	}
	a := s.propagation.clauses
	n, e := a.Size(c)
	if e != nil {
		return e
	}
	if n == 3 {
		return s.detachTerClause(c)
	}
	return s.detachNaryClause(c)
}
func (s *satSolverClauseLifecycle) detachNaryClause(c StoredClauseRef) error {
	a := s.propagation.clauses
	o, e := a.Offset(c)
	if e != nil {
		return e
	}
	for i := uint32(0); i < 2; i++ {
		l, e := a.Literal(c, i)
		if e != nil {
			return e
		}
		w, e := s.propagation.variables.watches.Get(l.Negated().index())
		if e != nil {
			return e
		}
		r, e := w.ref()
		if e != nil {
			return e
		}
		if _, e = w.owner.store.EraseClauseWatch(r, o); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSolverClauseLifecycle) detachTerClause(c StoredClauseRef) error {
	a := s.propagation.clauses
	for i := uint32(0); i < 3; i++ {
		l, e := a.Literal(c, i)
		if e != nil {
			return e
		}
		x, e := a.Literal(c, (i+1)%3)
		if e != nil {
			return e
		}
		y, e := a.Literal(c, (i+2)%3)
		if e != nil {
			return e
		}
		if i == 1 {
			x, y = y, x
		}
		w, e := s.propagation.variables.watches.Get(l.Negated().index())
		if e != nil {
			return e
		}
		r, e := w.ref()
		if e != nil {
			return e
		}
		if e = w.owner.store.EraseTernaryWatch(r, x, y); e != nil {
			return e
		}
	}
	return nil
}
