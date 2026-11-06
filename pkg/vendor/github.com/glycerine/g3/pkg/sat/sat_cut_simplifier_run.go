package sat

func (c *satCutSimplifierLive) ExtractLUTs() error {
	clauses, e := c.snapshotClauses()
	if e != nil {
		return e
	}
	bins, e := c.snapshotBinaries()
	if e != nil {
		return e
	}
	before := c.aig.NumCuts()
	remaining, e := c.aig.FindAndAddLUTs(clauses, bins)
	if e != nil {
		return e
	}
	c.stats.extractedLUTs += uint32(len(clauses) - len(remaining))
	_, e = c.aig.Compute()
	c.stats.numCuts = c.aig.NumCuts() - before
	c.stats.numCalls++
	return e
}
