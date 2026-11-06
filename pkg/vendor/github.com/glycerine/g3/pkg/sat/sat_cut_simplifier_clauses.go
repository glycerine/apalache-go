package sat

func (c *satCutSimplifierLive) snapshotClause(r StoredClauseRef) (satLUTClause, error) {
	n, e := c.solver.Clauses.Size(r)
	if e != nil {
		return satLUTClause{}, e
	}
	x := make([]Literal, n)
	for i := uint32(0); i < n; i++ {
		x[i], e = c.solver.Clauses.Literal(r, i)
		if e != nil {
			return satLUTClause{}, e
		}
	}
	learned, e := c.solver.Clauses.Learned(r)
	return satLUTClause{Literals: x, Learned: learned}, e
}
func (c *satCutSimplifierLive) snapshotClauses() ([]satLUTClause, error) {
	refs := append([]StoredClauseRef(nil), c.solver.Lifecycle.Original...)
	if c.config.learnedToAIG {
		refs = append(refs, c.solver.Lifecycle.learned...)
	}
	out := make([]satLUTClause, 0, len(refs))
	for _, r := range refs {
		removed, e := c.solver.Clauses.Removed(r)
		if e != nil {
			return nil, e
		}
		if removed {
			continue
		}
		x, e := c.snapshotClause(r)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
