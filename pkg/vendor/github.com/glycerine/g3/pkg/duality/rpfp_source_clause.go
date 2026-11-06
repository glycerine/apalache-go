package duality

// Source: muz/duality/duality_dl_interface.cpp, clauses and edge-to-clause map
// consumed by print_proof and extract_proof. The Go graph owns these references.

// OriginalClause returns an owned copy of the input Horn formula for an edge.
// The association survives dead-edge pruning and preprocessing of the formula.
func (g *RPFP) OriginalClause(edge *Edge) (*Expr, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return nil, ErrWrapperOwner
	}
	index, found := g.clauseIndices[edge]
	if !found || index < 0 || index >= len(g.sourceClauses) {
		return nil, ErrWrapperOwner
	}
	h, err := g.context.manager.RefGet(g.sourceClauses[index])
	if err != nil {
		return nil, err
	}
	return NewExpr(g.context, h)
}
