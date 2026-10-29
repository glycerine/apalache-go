package sat

// Source: src/sat/sat_simplifier.cpp:921–1083.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/stopwatch"
)

type satSimplifierBlocked struct {
	queue        *satVarQueue
	toRemove     *satClauseVector
	counter      int32
	reportActive bool
}

func (s *satSimplifier) blockedProcessVar(v BoolVar) (bool, error) {
	x, e := s.lifecycle.propagation.variables.IsExternal(v)
	if e != nil {
		return false, e
	}
	if x {
		return false, nil
	}
	x, e = s.lifecycle.propagation.variables.wasEliminated(v)
	return !x, e
}
func (s *satSimplifier) closeBlocked() error {
	b := &s.blocked
	if b.toRemove != nil {
		if e := b.toRemove.Close(); e != nil {
			return e
		}
		b.toRemove = nil
	}
	if b.queue != nil {
		if e := b.queue.Close(); e != nil {
			return e
		}
		b.queue = nil
	}
	if b.reportActive {
		if e := s.blockedReport.Stop(); e != nil {
			return e
		}
		b.reportActive = false
		if e := s.emitBlockedReport(); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSimplifier) eliminateBlocked() (err error) {
	s.traceBlockedStart()
	s.blockedReport = stopwatch.Stopwatch{}
	s.blockedReportBlocked = s.numBlocked
	if e := s.blockedReport.Start(); e != nil {
		return e
	}
	b := &s.blocked
	b.reportActive = true
	defer func() {
		if e := s.closeBlocked(); e != nil {
			if err == nil {
				err = e
			} else {
				err = errors.Join(err, e)
			}
		}
	}()
	b.counter = int32(s.blockedLimit)
	var e error
	b.queue, e = newSATSimplifierBlockedQueue(s)
	if e != nil {
		return e
	}
	b.toRemove, e = NewSATClauseVector(s.lifecycle.propagation.clauses, s.lifecycle.budget)
	if e != nil {
		return e
	}
	n, e := s.lifecycle.propagation.variables.numVarsChecked()
	if e != nil {
		return e
	}
	for v := uint32(0); v < n; v++ {
		yes, e := s.blockedProcessVar(BoolVar(v))
		if e != nil {
			return e
		}
		if yes {
			if e = b.queue.MkVar(BoolVar(v * 2)); e != nil {
				return e
			}
			if e = b.queue.MkVar(BoolVar(v*2 + 1)); e != nil {
				return e
			}
		}
	}
	for {
		empty, e := b.queue.emptyChecked()
		if e != nil {
			return e
		}
		if empty {
			break
		}
		if e = s.lifecycle.propagation.checkpoint(); e != nil {
			return e
		}
		if b.counter < 0 {
			return nil
		}
		idx, e := b.queue.NextVar()
		if e != nil {
			return e
		}
		if e = s.blockedProcess(Literal(idx)); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSimplifier) blockedNegativeOccurrences(l Literal) (result bool, err error) {
	view, e := s.useList.Get(l.Negated())
	if e != nil {
		return false, e
	}
	it, e := view.Begin()
	if e != nil {
		return false, e
	}
	defer func() {
		if e := it.Close(); err == nil {
			err = e
		}
	}()
	for !it.AtEnd() {
		c, e := it.Current()
		if e != nil {
			return false, e
		}
		n, e := s.lifecycle.propagation.clauses.Size(c)
		if e != nil {
			return false, e
		}
		s.blocked.counter = int32(uint32(s.blocked.counter) - n)
		n, e = s.lifecycle.propagation.clauses.Size(c)
		if e != nil {
			return false, e
		}
		i := uint32(0)
		for ; i < n; i++ {
			x, e := s.lifecycle.propagation.clauses.Literal(c, i)
			if e != nil {
				return false, e
			}
			yes, e := s.kernel.isMarked(x.Negated())
			if e != nil {
				return false, e
			}
			if yes {
				break
			}
		}
		if i == n {
			return false, nil
		}
		if e = it.Next(); e != nil {
			return false, e
		}
	}
	return true, nil
}
func (s *satSimplifier) allTautology(l Literal) (bool, error) {
	w, e := s.lifecycle.propagation.variables.watches.Get(uint32(l))
	if e != nil {
		return false, e
	}
	n, e := w.Size()
	if e != nil {
		return false, e
	}
	s.blocked.counter = int32(uint32(s.blocked.counter) - n)
	it, e := s.watchIterator(w)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < it.end; i++ {
		x, e := satPropagationCursorRead(&it, i)
		if e != nil {
			return false, e
		}
		if !x.BinaryNonLearned() {
			continue
		}
		lit, e := x.Literal()
		if e != nil {
			return false, e
		}
		yes, e := s.kernel.isMarked(lit.Negated())
		if e != nil {
			return false, e
		}
		if !yes {
			return false, nil
		}
	}
	return s.blockedNegativeOccurrences(l)
}
func (s *satSimplifier) blockedClauseOccurrences(l Literal, entry *satModelConversionEntryRef) (err error) {
	view, e := s.useList.Get(l)
	if e != nil {
		return e
	}
	it, e := view.Begin()
	if e != nil {
		return e
	}
	defer func() {
		if e := it.Close(); err == nil {
			err = e
		}
	}()
	p := s.lifecycle.propagation
	b := &s.blocked
	for !it.AtEnd() {
		c, e := it.Current()
		if e != nil {
			return e
		}
		n, e := p.clauses.Size(c)
		if e != nil {
			return e
		}
		b.counter = int32(uint32(b.counter) - n)
		if satStorageSourceDebug {
			yes, e := p.clauses.ContainsLiteral(c, l)
			if e != nil {
				return e
			}
			if !yes {
				return ErrSATAssertion
			}
		}
		if e = s.kernel.markAllBut(c, l); e != nil {
			return e
		}
		yes, e := s.allTautology(l)
		if e != nil {
			return e
		}
		if yes {
			s.traceBlockedClause(c)
			if *entry == (satModelConversionEntryRef{}) {
				*entry, e = p.converter.Mk(satModelBlockLit, l.variable())
				if e != nil {
					return e
				}
			}
			if e = b.toRemove.Push(c); e != nil {
				return e
			}
			s.numBlocked++
			if e = p.converter.InsertClause(*entry, p.clauses, c); e != nil {
				return e
			}
			n, e = p.clauses.Size(c)
			if e != nil {
				return e
			}
			for i := uint32(0); i < n; i++ {
				x, e := p.clauses.Literal(c, i)
				if e != nil {
					return e
				}
				if x != l {
					yes, e := s.blockedProcessVar(x.variable())
					if e != nil {
						return e
					}
					if yes {
						if e = s.blockedDecreased(b.queue, x.Negated()); e != nil {
							return e
						}
					}
				}
			}
		}
		if e = s.kernel.unmarkAll(c); e != nil {
			return e
		}
		if e = it.Next(); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSimplifier) removeBinaryHalf(l1, l2 Literal, learned bool) error {
	w, e := s.lifecycle.propagation.variables.watches.Get(uint32(l1.Negated()))
	if e != nil {
		return e
	}
	x, e := newSATBinaryWatch(l2, learned)
	if e != nil {
		return e
	}
	r, e := w.ref()
	if e != nil {
		return e
	}
	if satStorageSourceDebug {
		found, e := w.owner.store.Contains(r, x)
		if e != nil {
			return e
		}
		if !found {
			return ErrSATAssertion
		}
	}
	if _, e = w.owner.store.Erase(r, x); e != nil {
		return e
	}
	_, e = s.subBinTodo.Erase(newSATBinClause(l1, l2, learned))
	return e
}
func (s *satSimplifier) blockedProcess(l Literal) error {
	s.traceBlockedProcessing(l)
	yes, e := s.blockedProcessVar(l.variable())
	if e != nil {
		return e
	}
	if !yes {
		return nil
	}
	b := &s.blocked
	var entry satModelConversionEntryRef
	if e = b.toRemove.Reset(); e != nil {
		return e
	}
	if e = s.blockedClauseOccurrences(l, &entry); e != nil {
		return e
	}
	n := b.toRemove.Size()
	for i := uint32(0); i < n; i++ {
		c, e := b.toRemove.Get(i)
		if e != nil {
			return e
		}
		if e = s.removeClause(c); e != nil {
			return e
		}
	}
	p := s.lifecycle.propagation
	w, e := p.variables.watches.Get(uint32(l.Negated()))
	if e != nil {
		return e
	}
	count, e := w.Size()
	if e != nil {
		return e
	}
	b.counter = int32(uint32(b.counter) - count)
	it, e := s.watchIterator(w)
	if e != nil {
		return e
	}
	out := uint32(0)
	for i := uint32(0); i < it.end; i++ {
		x, e := satPropagationCursorRead(&it, i)
		if e != nil {
			return e
		}
		if !x.Binary() {
			if e = satPropagationCursorWrite(&it, out, x); e != nil {
				return e
			}
			out++
			continue
		}
		l2, e := x.Literal()
		if e != nil {
			return e
		}
		if e = s.kernel.markVisited(l2); e != nil {
			return e
		}
		yes, e := s.allTautology(l)
		if e != nil {
			return e
		}
		if yes {
			if entry == (satModelConversionEntryRef{}) {
				entry, e = p.converter.Mk(satModelBlockLit, l.variable())
				if e != nil {
					return e
				}
			}
			s.traceBlockedBinary(l2, l)
			learned, e := x.Learned()
			if e != nil {
				return e
			}
			if e = s.removeBinaryHalf(l2, l, learned); e != nil {
				return e
			}
			s.numBlocked++
			if e = s.blockedDecreased(b.queue, l2.Negated()); e != nil {
				return e
			}
			if e = p.converter.InsertBinary(entry, l, l2); e != nil {
				return e
			}
		} else {
			if e = satPropagationCursorWrite(&it, out, x); e != nil {
				return e
			}
			out++
		}
		if e = s.kernel.unmarkVisited(l2); e != nil {
			return e
		}
	}
	return p.finishPropagationList(&it, w, out)
}
