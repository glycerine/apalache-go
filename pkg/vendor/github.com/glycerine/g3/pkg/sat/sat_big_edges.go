package sat

// Source: src/sat/sat_big.cpp:72-91.

import "github.com/glycerine/g3/pkg/arena"

func (b *satBIG) initAddingEdges(numVars uint32, learned bool) error {
	if numVars > uint32(NullBoolVar) {
		return arena.ErrBounds
	}
	b.learned = learned
	b.numVars = numVars
	n := int(numVars * 2)
	b.dag = make([][]Literal, n)
	b.roots = make([]bool, n)
	for i := range b.roots {
		b.roots[i] = true
	}
	b.left, b.right, b.root, b.parent = nil, nil, nil, nil
	b.deleted = nil
	return nil
}

func (b *satBIG) addEdge(u, v Literal) error {
	if !b.validLiteral(u) || !b.validLiteral(v) {
		return arena.ErrBounds
	}
	b.dag[u.index()] = append(b.dag[u.index()], v)
	return nil
}

func (b *satBIG) addSolverEdge(u, v Literal) error {
	if err := b.addEdge(u, v); err != nil {
		return err
	}
	b.roots[v.index()] = false
	return nil
}

func (b *satBIG) shuffleEdges(edges []Literal) {
	for n := len(edges) - 1; n > 0; n-- {
		k := int(b.random.Next()) % (n + 1)
		edges[n], edges[k] = edges[k], edges[n]
	}
}

func (b *satBIG) doneAddingEdges() error {
	for _, edges := range b.dag {
		b.shuffleEdges(edges)
	}
	return b.initDFSNumbers()
}

func (b *satBIG) reinit() error { return b.doneAddingEdges() }
