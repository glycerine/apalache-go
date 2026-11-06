package sat

func (f *satAIGFinder) Run(clauses []satLUTClause, binaries []satLUTBinary) ([]satLUTClause, error) {
	if e := f.initGraph(clauses, binaries); e != nil {
		return nil, e
	}
	used := make([]bool, len(clauses))
	for i, c := range clauses {
		ok, e := f.findAND(c.Literals)
		if e != nil {
			return nil, e
		}
		used[i] = ok
	}
	if e := f.findITEs(clauses, used); e != nil {
		return nil, e
	}
	out := make([]satLUTClause, 0, len(clauses))
	for i, c := range clauses {
		if !used[i] {
			out = append(out, c)
		}
	}
	return out, nil
}
