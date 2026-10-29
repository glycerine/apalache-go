package sat

// resolve appends to the caller's real literal-vector storage. It marks only
// first-input literals, preserves repeated second-input literals, and leaves
// marks/output/counters at the exact completed statement on an error. Normal
// completion clears first-input marks even when the result is tautological.
func (k *satSimplifierKernel) resolve(c1, c2 satClauseWrapper, pivot Literal, out *satSolverLiteralStore, list satSolverLiteralListRef) (bool, error) {
	if e := k.open(); e != nil {
		return false, e
	}
	k.traceInvalidResolve(c1, c2, pivot)
	if satStorageSourceDebug {
		yes, e := c1.ContainsLiteral(k.allocator, pivot)
		if e != nil {
			return false, e
		}
		if !yes {
			return false, ErrSATAssertion
		}
		yes, e = c2.ContainsLiteral(k.allocator, pivot.Negated())
		if e != nil {
			return false, e
		}
		if !yes {
			return false, ErrSATAssertion
		}
	}
	res := true
	n, e := c1.Size(k.allocator)
	if e != nil {
		return false, e
	}
	k.elimCounter = int32(uint32(k.elimCounter) - n)
	for i := uint32(0); i < n; i++ {
		l, e := c1.Literal(k.allocator, i)
		if e != nil {
			return false, e
		}
		if l == pivot {
			continue
		}
		if e = k.markVisited(l); e != nil {
			return false, e
		}
		if e = out.Push(list, l); e != nil {
			return false, e
		}
	}
	notPivot := pivot.Negated()
	n, e = c2.Size(k.allocator)
	if e != nil {
		return false, e
	}
	k.elimCounter = int32(uint32(k.elimCounter) - n)
	for i := uint32(0); i < n; i++ {
		l, e := c2.Literal(k.allocator, i)
		if e != nil {
			return false, e
		}
		if l == notPivot {
			continue
		}
		complement, e := k.isMarked(l.Negated())
		if e != nil {
			return false, e
		}
		if complement {
			res = false
			break
		}
		marked, e := k.isMarked(l)
		if e != nil {
			return false, e
		}
		if !marked {
			if e = out.Push(list, l); e != nil {
				return false, e
			}
		}
	}
	n, e = c1.Size(k.allocator)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		l, e := c1.Literal(k.allocator, i)
		if e != nil {
			return false, e
		}
		if l == pivot {
			continue
		}
		if e = k.unmarkVisited(l); e != nil {
			return false, e
		}
	}
	return res, nil
}
