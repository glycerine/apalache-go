package sat

func (f *satAIGFinder) initGraph(clauses []satLUTClause, binaries []satLUTBinary) error {
	var vars uint32
	for _, c := range clauses {
		for _, l := range c.Literals {
			if uint32(l.variable())+1 > vars {
				vars = uint32(l.variable()) + 1
			}
		}
	}
	for _, b := range binaries {
		for _, l := range [...]Literal{b.First, b.Second} {
			if uint32(l.variable())+1 > vars {
				vars = uint32(l.variable()) + 1
			}
		}
	}
	if err := f.big.initAddingEdges(vars, true); err != nil {
		return err
	}
	f.binaries = make(map[[2]Literal]bool, len(binaries))
	for _, b := range binaries {
		f.binaries[satAIGPair(b.First, b.Second)] = true
		if err := f.big.addSolverEdge(b.First.Negated(), b.Second); err != nil {
			return err
		}
		if err := f.big.addSolverEdge(b.Second.Negated(), b.First); err != nil {
			return err
		}
	}
	return f.big.doneAddingEdges()
}
