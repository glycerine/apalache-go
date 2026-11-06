package sat

// Source: src/sat/sat_big.cpp:160-216.

type satBIGBinaryEdge struct {
	u, v    Literal
	learned bool
}

func (b *satBIG) solverBinaryEdges(l *satSolverClauseLifecycle) ([]satBIGBinaryEdge, error) {
	edges := make([]satBIGBinaryEdge, 0)
	for idx := uint32(0); idx < b.numLiterals(); idx++ {
		list, err := l.propagation.variables.watches.Get(idx)
		if err != nil {
			return nil, err
		}
		ref, err := list.ref()
		if err != nil {
			return nil, err
		}
		it, err := list.owner.store.Iter(ref)
		if err != nil {
			return nil, err
		}
		for {
			w, yes, e := it.Next()
			if e != nil {
				return nil, e
			}
			if !yes {
				break
			}
			if !w.Binary() {
				continue
			}
			isLearned, e := w.Learned()
			if e != nil {
				return nil, e
			}
			if b.learned != isLearned && b.learned {
				continue
			}
			v, e := w.Literal()
			if e != nil {
				return nil, e
			}
			edges = append(edges, satBIGBinaryEdge{Literal(idx), v, isLearned})
		}
	}
	return edges, nil
}

func (b *satBIG) hasBinaryEdge(edges []satBIGBinaryEdge, u, v Literal) bool {
	for _, edge := range edges {
		if edge.u == u && edge.v == v && !b.inDeleted(u.Negated(), v) {
			return true
		}
	}
	return false
}

func (b *satBIG) reduceTransitive(l *satSolverClauseLifecycle) (uint32, error) {
	if err := l.open(); err != nil {
		return 0, err
	}
	edges, err := b.solverBinaryEdges(l)
	if err != nil {
		return 0, err
	}
	b.deleted = make([][]Literal, b.numLiterals())
	var eliminated uint32
	for _, edge := range edges {
		if l.propagation.inconsistent {
			break
		}
		u, v := edge.u, edge.v
		if b.inDeleted(u.Negated(), v) || u == b.getParent(v) || u.Negated() == b.getParent(v) {
			continue
		}
		yes, e := b.safeReach(u, v)
		if e != nil {
			return eliminated, e
		}
		if !yes {
			continue
		}
		b.addDeleted(u.Negated(), v)
		if b.hasBinaryEdge(edges, u, v.Negated()) {
			if e = l.propagation.assign(u.Negated(), satJustification{}); e != nil {
				return eliminated, e
			}
		}
		if e = l.detachBinClause(u.Negated(), v, edge.learned); e != nil {
			return eliminated, e
		}
		eliminated++
	}
	if _, err = l.propagate(false); err != nil {
		return eliminated, err
	}
	return eliminated, nil
}
