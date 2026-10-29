package sat

// Source: src/sat/sat_simplifier.cpp:682–880.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/stopwatch"
)

func (s *satSimplifier) watchIterator(w satSolverWatchListView) (satSolverWatchIterator, error) {
	r, e := w.ref()
	if e != nil {
		return satSolverWatchIterator{}, e
	}
	return w.owner.store.Iter(r)
}
func (s *satSimplifier) markNotLearnedCore(w satSolverWatchListView, l Literal) error {
	it, e := s.watchIterator(w)
	if e != nil {
		return e
	}
	for i := uint32(0); i < it.end; i++ {
		x, e := satPropagationCursorRead(&it, i)
		if e != nil {
			return e
		}
		if !x.Binary() {
			continue
		}
		v, e := x.Literal()
		if e != nil {
			return e
		}
		if v != l {
			continue
		}
		learned, e := x.Learned()
		if e != nil {
			return e
		}
		if learned {
			if e = x.MarkNotLearned(); e != nil {
				return e
			}
			return satPropagationCursorWrite(&it, i, x)
		}
	}
	return nil
}
func (s *satSimplifier) markNotLearned(a, b Literal) error {
	w, e := s.lifecycle.propagation.variables.watches.Get(uint32(a.Negated()))
	if e != nil {
		return e
	}
	if e = s.markNotLearnedCore(w, b); e != nil {
		return e
	}
	w, e = s.lifecycle.propagation.variables.watches.Get(uint32(b.Negated()))
	if e != nil {
		return e
	}
	return s.markNotLearnedCore(w, a)
}
func (s *satSimplifier) subsumeBinaries() (bool, error) {
	// The actual solver RNG is shared with next_var/decide. This draw occurs even
	// when num_lits is zero, before the loop avoids the modulo-zero expression.
	init := s.decision.rng.Next()
	ws := s.lifecycle.propagation.variables.watches
	n := uint32(len(ws.lists))
	for i := uint32(0); i < n; i++ {
		idx := (i + init) % n
		wlist, e := ws.Get(idx)
		if e != nil {
			return false, e
		}
		l := Literal(idx).Negated()
		for j := uint32(0); ; j++ {
			count, e := wlist.Size()
			if e != nil {
				return false, e
			}
			if j >= count {
				break
			}
			w, e := wlist.Read(j)
			if e != nil {
				return false, e
			}
			if !w.Binary() {
				continue
			}
			l2, e := w.Literal()
			if e != nil {
				return false, e
			}
			if uint32(l) >= uint32(l2) {
				continue
			}
			learned, e := w.Learned()
			if e != nil {
				return false, e
			}
			if e = s.dummy.SetBinary(l, l2, learned); e != nil {
				return false, e
			}
			c := s.dummy.Get()
			if e = s.backSubsumption1(c); e != nil {
				return false, e
			}
			learned, e = w.Learned()
			if e != nil {
				return false, e
			}
			if learned {
				current, e := s.dummy.owner.Learned(c)
				if e != nil {
					return false, e
				}
				if !current {
					if satStorageSourceDebug {
						x, e := wlist.Read(j)
						if e != nil {
							return false, e
						}
						if x != w {
							return false, ErrSATAssertion
						}
					}
					x, e := wlist.Read(j)
					if e != nil {
						return false, e
					}
					s.traceMarkNotLearnedBinary(l2, x)
					if e = x.MarkNotLearned(); e != nil {
						return false, e
					}
					if e = wlist.Write(j, x); e != nil {
						return false, e
					}
					other, e := ws.Get(uint32(l2.Negated()))
					if e != nil {
						return false, e
					}
					if e = s.markNotLearnedCore(other, l); e != nil {
						return false, e
					}
				}
			}
			if s.lifecycle.propagation.inconsistent {
				return false, nil
			}
		}
		if s.kernel.subCounter < 0 {
			break
		}
	}
	return true, nil
}
func (s *satSimplifier) binaryFromClause(c StoredClauseRef) error {
	a := s.lifecycle.propagation.clauses
	l1, e := a.Literal(c, 0)
	if e != nil {
		return e
	}
	l2, e := a.Literal(c, 1)
	if e != nil {
		return e
	}
	learned, e := a.Learned(c)
	if e != nil {
		return e
	}
	if e = s.lifecycle.mkBinClause(l1, l2, learned); e != nil {
		return e
	}
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
}
func (s *satSimplifier) subsume() (err error) {
	s.subReport = stopwatch.Stopwatch{}
	s.subReportSubsumed, s.subReportSubRes = s.numSubsumed, s.numSubRes
	if e := s.subReport.Start(); e != nil {
		return e
	}
	s.subReportActive = true
	defer func() {
		if e := s.closeSubsumeReport(); e != nil {
			err = errors.Join(err, e)
		}
	}()
	if e := s.eliminateDuplicateBinaries(); e != nil {
		return e
	}
	if _, e := s.subsumeBinaries(); e != nil {
		return e
	} // source intentionally ignores bool
	s.traceSubsumeSolverDisplay(813)
	p := s.lifecycle.propagation
	for {
		s.traceSubsumptionTodoSize()
		s.kernel.subCounter = int32(uint32(s.kernel.subCounter) - s.subBinTodo.Size())
		for s.subBinTodo.Size() != 0 {
			if e := p.checkpoint(); e != nil {
				return e
			}
			b, e := s.subBinTodo.Get(s.subBinTodo.Size() - 1)
			if e != nil {
				return e
			}
			if e = s.dummy.SetBinary(b.Literal1(), b.Literal2(), b.Learned()); e != nil {
				return e
			}
			if _, e = s.subBinTodo.Pop(); e != nil {
				return e
			}
			c := s.dummy.Get()
			learned, e := s.dummy.owner.Learned(c)
			if e != nil {
				return e
			}
			if e = s.backSubsumption1(c); e != nil {
				return e
			}
			if learned {
				now, e := s.dummy.owner.Learned(c)
				if e != nil {
					return e
				}
				if !now {
					a, e := s.dummy.owner.Literal(c, 0)
					if e != nil {
						return e
					}
					b, e := s.dummy.owner.Literal(c, 1)
					if e != nil {
						return e
					}
					if e = s.markNotLearned(a, b); e != nil {
						return e
					}
				}
			}
		}
		if e := p.checkpoint(); e != nil {
			return e
		}
		s.traceSubsumeSolverDisplay(832)
		if s.subTodo.Empty() {
			var e error
			s.lastSubTrailSize, e = p.variables.trail.Size(p.variables.trailRef)
			return e
		}
		if s.kernel.subCounter < 0 {
			return nil
		}
		c, e := s.subTodo.Pop()
		if e != nil {
			return e
		}
		if e = p.clauses.UnmarkStrengthened(c); e != nil {
			return e
		}
		if s.kernel.subCounter == -1<<31 {
			return ErrSATSignedCounterDomain
		}
		s.kernel.subCounter--
		s.traceSubsumptionTodoClause(845, "next: ", c)
		trail, e := p.variables.trail.Size(p.variables.trailRef)
		if e != nil {
			return e
		}
		if trail > s.lastSubTrailSize {
			yes, e := s.cleanupClause(c, true)
			if e != nil {
				return e
			}
			if yes {
				if e = s.removeClause(c); e != nil {
					return e
				}
				continue
			}
			n, e := p.clauses.Size(c)
			if e != nil {
				return e
			}
			switch n {
			case 0:
				p.setConflict(satJustification{}, DefaultLiteral())
				return nil
			case 1:
				l, e := p.clauses.Literal(c, 0)
				if e != nil {
					return e
				}
				if e = s.propagateUnit(l); e != nil {
					return e
				}
				continue
			case 2:
				s.traceSubsumptionTodoClause(863, "clause became binary: ", c)
				if e = s.binaryFromClause(c); e != nil {
					return e
				}
				continue
			}
		}
		s.traceSubsumptionTodoClause(870, "using: ", c)
		if e = s.backSubsumption1(c); e != nil {
			return e
		}
	}
}

func (s *satSimplifier) closeSubsumeReport() error {
	if !s.subReportActive {
		return nil
	}
	if e := s.subReport.Stop(); e != nil {
		return e
	}
	s.subReportActive = false
	return s.emitSubsumeReport()
}
