package sat

func (c *satCutSimplifierLive) snapshotBinaries() ([]satLUTBinary, error) {
	pairs, e := c.solver.appendBinaryClauses(nil, true)
	if e != nil {
		return nil, e
	}
	out := make([]satLUTBinary, len(pairs))
	for i, p := range pairs {
		out[i] = satLUTBinary{First: p.Literal1(), Second: p.Literal2()}
	}
	return out, nil
}
