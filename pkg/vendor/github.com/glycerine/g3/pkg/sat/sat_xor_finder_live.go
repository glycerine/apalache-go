package sat

func (c *satCutSimplifierLive) ExtractXORs() error {
	clauses, e := c.snapshotClauses()
	if e != nil {
		return e
	}
	bins, e := c.snapshotBinaries()
	if e != nil {
		return e
	}
	_, e = newSATXORFinder(func(xs []Literal) error {
		for _, lit := range xs {
			if err := c.solver.Variables.setExternal(lit.variable(), true); err != nil {
				return err
			}
		}
		max := 0
		for i := 1; i < len(xs); i++ {
			if xs[i].variable() > xs[max].variable() {
				max = i
			}
		}
		head := xs[max].Negated()
		args := make([]Literal, 0, len(xs)-1)
		for i := len(xs) - 1; i >= 0; i-- {
			if i != max {
				args = append(args, xs[i])
			}
		}
		if e := c.aig.AddNode(head, satAIGXor, args); e != nil {
			return e
		}
		c.stats.extractedXors++
		return nil
	}).Run(clauses, bins)
	return e
}
