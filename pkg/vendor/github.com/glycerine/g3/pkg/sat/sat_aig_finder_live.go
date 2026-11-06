package sat

func (c *satCutSimplifierLive) ExtractAIGs() error {
	clauses, e := c.snapshotClauses()
	if e != nil {
		return e
	}
	bins, e := c.snapshotBinaries()
	if e != nil {
		return e
	}
	f := newSATAIGFinder(func(h Literal, args []Literal) error {
		if err := c.aig.AddNode(h, satAIGAnd, args); err != nil {
			return err
		}
		c.stats.extractedAnds++
		return nil
	}, func(h, cond, yes, no Literal) error {
		if err := c.aig.AddNode(h, satAIGITE, []Literal{cond, yes, no}); err != nil {
			return err
		}
		c.stats.extractedITEs++
		return nil
	})
	_, e = f.Run(clauses, bins)
	return e
}
