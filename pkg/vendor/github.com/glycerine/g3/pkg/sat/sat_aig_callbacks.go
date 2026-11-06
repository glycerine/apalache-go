package sat

func (a *satAIGCuts) SetOnClauseAdd(f satAIGClauseUpdate) {
	a.onClauseAdd = f
	if f == nil {
		a.onCutAdd = nil
		return
	}
	a.onCutAdd = func(v uint32, c satCut) { a.cut2def(f, c, MakeLiteral(BoolVar(v), false)) }
}
func (a *satAIGCuts) SetOnClauseDel(f satAIGClauseUpdate) {
	a.onClauseDel = f
	if f == nil {
		a.onCutDel = nil
		return
	}
	a.onCutDel = func(v uint32, c satCut) { a.cut2def(f, c, MakeLiteral(BoolVar(v), false)) }
}
func (a *satAIGCuts) onNodeAdd(v uint32, n satAIGNode) {
	if a.onClauseAdd != nil {
		a.node2def(a.onClauseAdd, n, MakeLiteral(BoolVar(v), false))
	}
}
func (a *satAIGCuts) onNodeDel(v uint32, n satAIGNode) {
	if a.onClauseDel != nil {
		a.node2def(a.onClauseDel, n, MakeLiteral(BoolVar(v), false))
	}
}

func emitSATClause(f satAIGClauseUpdate, clause ...Literal) {
	x := append([]Literal(nil), clause...)
	f(x)
}

func (a *satAIGCuts) cut2def(f satAIGClauseUpdate, c satCut, r Literal) {
	for assignment := uint32(0); assignment < uint32(1)<<c.size; assignment++ {
		clause := make([]Literal, 0, c.size+1)
		for j := uint32(0); j < c.size; j++ {
			clause = append(clause, MakeLiteral(BoolVar(c.elems[j]), assignment&(1<<j) != 0))
		}
		rr := r
		if c.Table()&(uint64(1)<<assignment) == 0 {
			rr = rr.Negated()
		}
		clause = append(clause, rr)
		f(clause)
	}
}
