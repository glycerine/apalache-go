package sat

// Source: src/sat/sat_simplifier.cpp; src/sat/sat_simplifier.h.

func (k *satSimplifierKernel) occurrenceCount(l Literal) (uint32, error) {
	v, e := k.useList.Get(l)
	if e != nil {
		return 0, e
	}
	return v.Size()
}
func (k *satSimplifierKernel) minOccurrenceVar(c StoredClauseRef) (BoolVar, error) {
	if e := k.open(); e != nil {
		return 0, e
	}
	bestLit, e := k.clauseLiteral(c, 0)
	if e != nil {
		return 0, e
	}
	p, e := k.occurrenceCount(bestLit)
	if e != nil {
		return 0, e
	}
	q, e := k.occurrenceCount(bestLit.Negated())
	if e != nil {
		return 0, e
	}
	best := p + q
	n, e := k.clauseSize(c)
	if e != nil {
		return 0, e
	}
	for i := uint32(1); i < n; i++ {
		l, e := k.clauseLiteral(c, i)
		if e != nil {
			return 0, e
		}
		p, e := k.occurrenceCount(l)
		if e != nil {
			return 0, e
		}
		q, e := k.occurrenceCount(l.Negated())
		if e != nil {
			return 0, e
		}
		count := p + q
		if count < best {
			bestLit = l
			best = count
		}
	}
	return bestLit.variable(), nil
}
func (k *satSimplifierKernel) minOccurrenceLiteral(c StoredClauseRef) (Literal, error) {
	if e := k.open(); e != nil {
		return 0, e
	}
	bestLit, e := k.clauseLiteral(c, 0)
	if e != nil {
		return 0, e
	}
	best, e := k.occurrenceCount(bestLit)
	if e != nil {
		return 0, e
	}
	n, e := k.clauseSize(c)
	if e != nil {
		return 0, e
	}
	for i := uint32(1); i < n; i++ {
		l, e := k.clauseLiteral(c, i)
		if e != nil {
			return 0, e
		}
		count, e := k.occurrenceCount(l)
		if e != nil {
			return 0, e
		}
		if count < best {
			bestLit = l
			best = count
		}
	}
	return bestLit, nil
}
func (k *satSimplifierKernel) subsumes1(c1, c2 StoredClauseRef, out *Literal) (bool, error) {
	if e := k.open(); e != nil {
		return false, e
	}
	// A Go nil output is outside the source reference domain, checked before use.
	if out == nil {
		return false, ErrSATOwner
	}
	n2, e := k.clauseSize(c2)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n2; i++ {
		l, e := k.clauseLiteral(c2, i)
		if e != nil {
			return false, e
		}
		if e = k.markVisited(l); e != nil {
			return false, e
		}
	}
	r := true
	*out = DefaultLiteral()
	n1, e := k.clauseSize(c1)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n1; i++ {
		l, e := k.clauseLiteral(c1, i)
		if e != nil {
			return false, e
		}
		marked, e := k.isMarked(l)
		if e != nil {
			return false, e
		}
		if !marked {
			if *out == DefaultLiteral() {
				opposite, e := k.isMarked(l.Negated())
				if e != nil {
					return false, e
				}
				if opposite {
					*out = l.Negated()
					continue
				}
			}
			*out = DefaultLiteral()
			r = false
			break
		}
	}
	for i := uint32(0); i < n2; i++ {
		l, e := k.clauseLiteral(c2, i)
		if e != nil {
			return false, e
		}
		if e = k.unmarkVisited(l); e != nil {
			return false, e
		}
	}
	return r, nil
}
func (k *satSimplifierKernel) subsumes0(c1, c2 StoredClauseRef) (bool, error) {
	if e := k.open(); e != nil {
		return false, e
	}
	n2, e := k.clauseSize(c2)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n2; i++ {
		l, e := k.clauseLiteral(c2, i)
		if e != nil {
			return false, e
		}
		if e = k.markVisited(l); e != nil {
			return false, e
		}
	}
	r := true
	n1, e := k.clauseSize(c1)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n1; i++ {
		l, e := k.clauseLiteral(c1, i)
		if e != nil {
			return false, e
		}
		yes, e := k.isMarked(l)
		if e != nil {
			return false, e
		}
		if !yes {
			r = false
			break
		}
	}
	for i := uint32(0); i < n2; i++ {
		l, e := k.clauseLiteral(c2, i)
		if e != nil {
			return false, e
		}
		if e = k.unmarkVisited(l); e != nil {
			return false, e
		}
	}
	return r, nil
}
func (k *satSimplifierKernel) candidate(c1, c2 StoredClauseRef) (bool, error) {
	if c2 == c1 {
		return false, nil
	}
	n1, e := k.clauseSize(c1)
	if e != nil {
		return false, e
	}
	n2, e := k.clauseSize(c2)
	if e != nil {
		return false, e
	}
	if n1 > n2 {
		return false, nil
	}
	a1, e := k.clauseApprox(c1)
	if e != nil {
		return false, e
	}
	a2, e := k.clauseApprox(c2)
	if e != nil {
		return false, e
	}
	if !a1.approxSubset(a2) {
		return false, nil
	}
	// The size operands are unsigned: preserve modulo32 before conversion to int.
	k.subCounter = int32(uint32(k.subCounter) - (n1 + n2))
	return true, nil
}
func (k *satSimplifierKernel) collectSubsumed1Core(c StoredClauseRef, out *satClauseVector, lits *satSolverLiteralStore, list satSolverLiteralListRef, target Literal) (err error) {
	if e := k.open(); e != nil {
		return e
	}
	view, e := k.useList.Get(target)
	if e != nil {
		return e
	}
	it, e := view.Begin()
	if e != nil {
		return e
	}
	// Actual source iterator destructor completes lazy compaction on every lexical
	// exit. A guarded destructor error does not replace the original refusal.
	defer func() {
		if e := it.Close(); err == nil {
			err = e
		}
	}()
	for !it.AtEnd() {
		c2, e := it.Current()
		if e != nil {
			return e
		}
		if satStorageSourceDebug {
			dead, e := k.clauseRemoved(c2)
			if e != nil {
				return e
			}
			if dead {
				return ErrSATAssertion
			}
		}
		yes, e := k.candidate(c, c2)
		if e != nil {
			return e
		}
		if yes {
			var l Literal
			yes, e = k.subsumes1(c, c2, &l)
			if e != nil {
				return e
			}
			if yes {
				if e = out.Push(c2); e != nil {
					return e
				}
				if e = lits.Push(list, l); e != nil {
					return e
				}
			}
		}
		if e = it.Next(); e != nil {
			return e
		}
	}
	return nil
}
func (k *satSimplifierKernel) collectSubsumed1(c StoredClauseRef, out *satClauseVector, lits *satSolverLiteralStore, list satSolverLiteralListRef) error {
	v, e := k.minOccurrenceVar(c)
	if e != nil {
		return e
	}
	if e = k.collectSubsumed1Core(c, out, lits, list, MakeLiteral(v, false)); e != nil {
		return e
	}
	return k.collectSubsumed1Core(c, out, lits, list, MakeLiteral(v, true))
}
func (k *satSimplifierKernel) collectSubsumed0Core(c StoredClauseRef, out *satClauseVector, target Literal) (err error) {
	if e := k.open(); e != nil {
		return e
	}
	view, e := k.useList.Get(target)
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
	for !it.AtEnd() {
		c2, e := it.Current()
		if e != nil {
			return e
		}
		if satStorageSourceDebug {
			dead, e := k.clauseRemoved(c2)
			if e != nil {
				return e
			}
			if dead {
				return ErrSATAssertion
			}
		}
		yes, e := k.candidate(c, c2)
		if e != nil {
			return e
		}
		if yes {
			yes, e = k.subsumes0(c, c2)
			if e != nil {
				return e
			}
			if yes {
				if e = out.Push(c2); e != nil {
					return e
				}
			}
		}
		if e = it.Next(); e != nil {
			return e
		}
	}
	return nil
}
func (k *satSimplifierKernel) collectSubsumed0(c StoredClauseRef, out *satClauseVector) error {
	l, e := k.minOccurrenceLiteral(c)
	if e != nil {
		return e
	}
	return k.collectSubsumed0Core(c, out, l)
}
