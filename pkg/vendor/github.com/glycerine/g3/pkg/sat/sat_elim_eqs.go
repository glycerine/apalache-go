package sat

// Source: src/sat/sat_elim_eqs.cpp; src/sat/sat_elim_eqs.h.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

type satElimEqs struct {
	self                                       *satElimEqs
	closing                                    bool
	solver                                     *Solver
	lifecycle                                  *satSolverClauseLifecycle
	borrowedRoots                              []Literal
	borrowedElim                               []BoolVar
	cursor, out, end, drainPrefix, drainSuffix uint32
	header                                     uint64
	learned, interrupted, running, closed      bool
}

func newSATElimEqs(l *satSolverClauseLifecycle) (*satElimEqs, error) {
	if e := l.open(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(satElimEqs{}).Size())
	if e := l.budget.Reserve(h); e != nil {
		return nil, e
	}
	e := &satElimEqs{lifecycle: l, header: h}
	e.self = e
	return e, nil
}
func (e *satElimEqs) Close() error {
	if e == nil || e.self != e || e.lifecycle == nil {
		return ErrSATOwner
	}
	if e.closed {
		return nil
	}
	e.closing = true
	if err := e.cleanupInterrupted(); err != nil {
		return err
	}
	e.lifecycle.budget.Release(e.header)
	e.header = 0
	e.closed = true
	return nil
}
func (e *satElimEqs) root(v BoolVar) (Literal, error) {
	if uint64(v) >= uint64(len(e.borrowedRoots)) {
		return 0, arena.ErrBounds
	}
	return e.borrowedRoots[v], nil
}
func (e *satElimEqs) norm(l Literal) (Literal, error) {
	r, err := e.root(l.variable())
	if l.sign() {
		r = r.Negated()
	}
	return r, err
}
func (e *satElimEqs) run(roots []Literal, toElim []BoolVar) error {
	e.borrowedRoots = roots
	e.borrowedElim = toElim
	defer func() { e.borrowedRoots = nil; e.borrowedElim = nil }()
	return e.execute()
}
func (e *satElimEqs) execute() error {
	if e == nil || e.self != e || e.lifecycle == nil {
		return ErrSATOwner
	}
	if e.closed || e.closing || e.interrupted {
		return arena.ErrClosed
	}
	if err := e.lifecycle.open(); err != nil {
		return err
	}
	if err := e.cleanupBinWatches(); err != nil {
		return err
	}
	e.traceBinCleanup()
	if err := e.cleanupClauses(false); err != nil {
		return err
	}
	p := e.lifecycle.propagation
	if p.inconsistent {
		return nil
	}
	if err := e.cleanupClauses(true); err != nil {
		return err
	}
	if p.inconsistent {
		return nil
	}
	if err := e.saveElim(); err != nil {
		return err
	}
	if _, err := p.propagateCore(false); err != nil {
		return err
	}
	if satStorageSourceDebug {
		return e.checkClauses()
	}
	return nil
}
func (e *satElimEqs) cleanupBinWatches() error {
	p := e.lifecycle.propagation
	v := p.variables
	n := uint32(len(v.watches.lists))
	for idx := uint32(0); idx < n; idx++ {
		list, err := v.watches.Get(idx)
		if err != nil {
			return err
		}
		l1 := Literal(idx).Negated()
		r1, err := e.norm(l1)
		if err != nil {
			return err
		}
		ref, err := list.ref()
		if err != nil {
			return err
		}
		it, err := list.owner.store.Iter(ref)
		if err != nil {
			return err
		}
		out := uint32(0)
		for pos := uint32(0); pos < it.end; pos++ {
			w, err := satPropagationCursorRead(&it, pos)
			if err != nil {
				return err
			}
			if w.Kind() == 0 {
				l2, err := w.Literal()
				if err != nil {
					return err
				}
				r2, err := e.norm(l2)
				if err != nil {
					return err
				}
				if r1 == r2 {
					if err = p.assign(r1, satJustification{}); err != nil {
						return err
					}
					if p.inconsistent {
						return nil
					}
					continue
				}
				if r1 == r2.Negated() {
					continue
				}
				if l1 != r1 {
					target, err := v.watches.Get(r1.Negated().index())
					if err != nil {
						return err
					}
					learned, err := w.Learned()
					if err != nil {
						return err
					}
					nw, err := newSATBinaryWatch(r2, learned)
					if err != nil {
						return err
					}
					if err = target.Push(nw); err != nil {
						return err
					}
					continue
				}
				if err = w.SetLiteral(r2); err != nil {
					return err
				}
				if err = satPropagationCursorWrite(&it, pos, w); err != nil {
					return err
				}
			}
			if err = satPropagationCursorWrite(&it, out, w); err != nil {
				return err
			}
			out++
		}
		if err = p.finishPropagationList(&it, list, out); err != nil {
			return err
		}
	}
	return nil
}
func (e *satElimEqs) vector() *[]StoredClauseRef {
	if e.learned {
		return &e.lifecycle.learned
	}
	return &e.lifecycle.Original
}
func (e *satElimEqs) begin(learned bool) error {
	l := e.lifecycle
	if l.eqContinuation != nil {
		return arena.ErrClosed
	}
	e.learned = learned
	e.cursor = 0
	e.out = 0
	e.drainPrefix = 0
	e.drainSuffix = 0
	e.end = uint32(len(*e.vector()))
	e.interrupted = true
	e.running = true
	l.eqContinuation = e
	return nil
}
func (e *satElimEqs) finish(n uint32) {
	vec := e.vector()
	*vec = (*vec)[:n]
	e.interrupted = false
	e.lifecycle.eqContinuation = nil
}
func (e *satElimEqs) cleanupClauses(learned bool) error {
	if err := e.begin(learned); err != nil {
		return err
	}
	defer func() { e.running = false }()
	l := e.lifecycle
	p := l.propagation
	a := p.clauses
	vec := e.vector()
	for i := uint32(0); i < e.end; i++ {
		c := (*vec)[i]
		e.traceClause(81, "processing: ", c, "")
		sz, err := a.Size(c)
		if err != nil {
			return err
		}
		k := uint32(0)
		for ; k < sz; k++ {
			lit, err := a.Literal(c, k)
			if err != nil {
				return err
			}
			r, err := e.norm(lit)
			if err != nil {
				return err
			}
			if lit != r {
				break
			}
		}
		if k == sz {
			(*vec)[e.out] = c
			e.out++
			e.cursor = i + 1
			continue
		}
		frozen, err := a.Frozen(c)
		if err != nil {
			return err
		}
		if !frozen {
			if err = l.detachClause(c); err != nil {
				return err
			}
		}
		for k = 0; k < sz; k++ {
			lit, err := a.Literal(c, k)
			if err != nil {
				return err
			}
			if satStorageSourceDebug {
				elim, err := p.variables.wasEliminated(lit.variable())
				if err != nil {
					return err
				}
				if elim {
					return ErrSATAssertion
				}
			}
			r, err := e.norm(lit)
			if err != nil {
				return err
			}
			if err = a.SetLiteral(c, k, r); err != nil {
				return err
			}
		}
		if err = e.sortClause(c, sz); err != nil {
			return err
		}
		e.traceClause(104, "after normalization/sorting: ", c, "")
		prev := DefaultLiteral()
		j := uint32(0)
		for k = 0; k < sz; k++ {
			lit, err := a.Literal(c, k)
			if err != nil {
				return err
			}
			if lit == prev {
				continue
			}
			if lit == prev.Negated() {
				break
			}
			prev = lit
			val, err := p.variables.valueLiteral(lit)
			if err != nil {
				return err
			}
			if val == lbool.True {
				break
			}
			if val == lbool.False {
				continue
			}
			if err = a.SetLiteral(c, j, lit); err != nil {
				return err
			}
			j++
		}
		if k < sz {
			if err = l.delClause(c); err != nil {
				return err
			}
			e.cursor = i + 1
			continue
		}
		if j == 0 {
			p.setConflict(satJustification{}, DefaultLiteral())
			for ; i < e.end; i++ {
				(*vec)[e.out] = (*vec)[i]
				e.out++
				e.cursor = i + 1
			}
			e.finish(e.out)
			return nil
		}
		e.traceClause(138, "after removing duplicates: ", c, " j: "+strconv.FormatUint(uint64(j), 10))
		if j < sz {
			err = a.Shrink(c, j)
		} else {
			err = a.UpdateApprox(c)
		}
		if err != nil {
			return err
		}
		if satStorageSourceDebug {
			n, err := a.Size(c)
			if err != nil {
				return err
			}
			if n != j {
				return ErrSATAssertion
			}
			for k := uint32(0); k < n; k++ {
				lit, err := a.Literal(c, k)
				if err != nil {
					return err
				}
				r, err := e.norm(lit)
				if err != nil {
					return err
				}
				if lit != r {
					return ErrSATAssertion
				}
			}
			if j < 1 {
				return ErrSATAssertion
			}
		}
		switch j {
		case 1:
			lit, err := a.Literal(c, 0)
			if err != nil {
				return err
			}
			if err = p.assign(lit, satJustification{}); err != nil {
				return err
			}
			if err = l.delClause(c); err != nil {
				return err
			}
			e.cursor = i + 1
		case 2:
			x, err := a.Literal(c, 0)
			if err != nil {
				return err
			}
			y, err := a.Literal(c, 1)
			if err != nil {
				return err
			}
			learn, err := a.Learned(c)
			if err != nil {
				return err
			}
			if err = l.mkBinClause(x, y, learn); err != nil {
				return err
			}
			if err = l.delClause(c); err != nil {
				return err
			}
			e.cursor = i + 1
		default:
			if satStorageSourceDebug && (*vec)[i] != c {
				return ErrSATAssertion
			}
			(*vec)[e.out] = c
			e.out++
			e.cursor = i + 1
			// Source rereads c.frozen() after compaction, at the attach site.
			frozen, err = a.Frozen(c)
			if err != nil {
				return err
			}
			if !frozen {
				if err = l.attachClause(c); err != nil {
					return err
				}
			}
		}
	}
	e.finish(e.out)
	return nil
}

// Concrete in-place heapsort adapter for source std::sort(literal*,literal*).
// Preserves unsigned ascending raw literal order with O(n log n) work and no
// snapshots/allocations. Equal words are indistinguishable. The selected C++
// library's comparison/swap schedule and malformed-read prefixes are not claimed.
func (e *satElimEqs) sortClause(c StoredClauseRef, n uint32) error {
	if n < 2 {
		return nil
	}
	for i := n / 2; i > 0; i-- {
		if err := e.siftClause(c, i-1, n); err != nil {
			return err
		}
	}
	a := e.lifecycle.propagation.clauses
	for end := n - 1; end > 0; end-- {
		x, err := a.Literal(c, 0)
		if err != nil {
			return err
		}
		y, err := a.Literal(c, end)
		if err != nil {
			return err
		}
		if err = a.SetLiteral(c, 0, y); err != nil {
			return err
		}
		if err = a.SetLiteral(c, end, x); err != nil {
			return err
		}
		if err = e.siftClause(c, 0, end); err != nil {
			return err
		}
	}
	return nil
}
func (e *satElimEqs) siftClause(c StoredClauseRef, root, end uint32) error {
	a := e.lifecycle.propagation.clauses
	// root<end/2 proves 2*root+1 has no uint32 overflow and is below end.
	for root < end/2 {
		child := root*2 + 1
		if child >= end {
			return nil
		}
		x, err := a.Literal(c, child)
		if err != nil {
			return err
		}
		if child+1 < end {
			y, err := a.Literal(c, child+1)
			if err != nil {
				return err
			}
			if y > x {
				child++
				x = y
			}
		}
		value, err := a.Literal(c, root)
		if err != nil {
			return err
		}
		if value >= x {
			return nil
		}
		if err = a.SetLiteral(c, root, x); err != nil {
			return err
		}
		if err = a.SetLiteral(c, child, value); err != nil {
			return err
		}
		root = child
	}
	return nil
}
func (e *satElimEqs) saveElim() error {
	p := e.lifecycle.propagation
	for _, v := range e.borrowedElim {
		lit := Literal(uint32(v) << 1)
		r, err := e.root(v)
		if err != nil {
			return err
		}
		if satStorageSourceDebug && v == r.variable() {
			return ErrSATAssertion
		}
		ext, err := p.variables.IsExternal(v)
		if err != nil {
			return err
		}
		if ext {
			if err = e.lifecycle.mkBinClause(lit.Negated(), r, false); err != nil {
				return err
			}
			if err = e.lifecycle.mkBinClause(lit, r.Negated(), false); err != nil {
				return err
			}
		} else {
			entry, err := p.converter.Mk(satModelElimVar, v)
			if err != nil {
				return err
			}
			e.traceSaveElim(v, lit, r)
			if err = p.variables.setEliminated(v, true); err != nil {
				return err
			}
			if err = p.converter.InsertBinary(entry, lit.Negated(), r); err != nil {
				return err
			}
			if err = p.converter.InsertBinary(entry, lit, r.Negated()); err != nil {
				return err
			}
		}
	}
	return nil
}
func (e *satElimEqs) checkClauses() error {
	p := e.lifecycle.propagation
	for _, vec := range [2][]StoredClauseRef{e.lifecycle.Original, e.lifecycle.learned} {
		for _, c := range vec {
			n, err := p.clauses.Size(c)
			if err != nil {
				return err
			}
			for i := uint32(0); i < n; i++ {
				lit, err := p.clauses.Literal(c, i)
				if err != nil {
					return err
				}
				elim, err := p.variables.wasEliminated(lit.variable())
				if err != nil {
					return err
				}
				e.traceEliminatedClauseLiteral(elim, lit, c)
				if elim {
					return ErrSATAssertion
				}
			}
		}
	}
	return nil
}
func (e *satElimEqs) cleanupInterrupted() error {
	if !e.interrupted {
		return nil
	}
	if e.lifecycle.eqContinuation != e {
		return ErrSATOwner
	}
	vec := e.vector()
	if uint32(len(*vec)) < e.end {
		return arena.ErrBounds
	}
	a := e.lifecycle.propagation.clauses
	for e.drainPrefix < e.out {
		if err := a.Delete((*vec)[e.drainPrefix]); err != nil {
			return err
		}
		e.drainPrefix++
	}
	if e.drainSuffix < e.cursor {
		e.drainSuffix = e.cursor
	}
	for e.drainSuffix < e.end {
		if err := a.Delete((*vec)[e.drainSuffix]); err != nil {
			return err
		}
		e.drainSuffix++
	}
	e.finish(0)
	return nil
}
