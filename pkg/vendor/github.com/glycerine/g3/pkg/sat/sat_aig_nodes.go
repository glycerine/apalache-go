package sat

func (a *satAIGCuts) nodeEqual(x, y satAIGNode) bool {
	if x.valid() != y.valid() {
		return false
	}
	if !x.valid() {
		return true
	}
	if x.op != y.op || x.sign != y.sign || x.size != y.size {
		return false
	}
	for i := uint32(0); i < x.size; i++ {
		if a.child(x, i) != a.child(y, i) {
			return false
		}
	}
	return true
}
func (a *satAIGCuts) similar(x, y satAIGNode) bool {
	if !x.isLUT() || y.isLUT() || x.size != y.size {
		return false
	}
	for i := uint32(0); i < x.size; i++ {
		if a.child(x, i).variable() != a.child(y, i).variable() {
			return false
		}
	}
	return true
}
func (a *satAIGCuts) addNode(v uint32, n satAIGNode) error {
	for i := uint32(0); i < n.size; i++ {
		if err := a.AddVar(uint32(a.child(n, i).variable())); err != nil {
			return err
		}
	}
	if len(a.nodes[v]) == 0 || n.isConst() {
		a.nodes[v] = []satAIGNode{n}
		a.onNodeAdd(v, n)
		if err := a.initCutSet(v); err != nil {
			return err
		}
		if n.isConst() {
			return a.augmentAIG0(v, n, a.cuts[v])
		}
		a.touch(v)
		return nil
	}
	if a.nodes[v][0].isConst() || !a.insertAux(v, n) {
		a.literals = a.literals[:n.offset]
		return nil
	}
	return nil
}
func (a *satAIGCuts) insertAux(v uint32, n satAIGNode) bool {
	if !a.config.full {
		return false
	}
	for _, x := range a.nodes[v] {
		if a.nodeEqual(n, x) || a.similar(n, x) {
			return false
		}
	}
	if uint32(len(a.nodes[v])) < a.config.maxAux {
		a.onNodeAdd(v, n)
		a.nodes[v] = append(a.nodes[v], n)
		a.touch(v)
		return true
	}
	best := make([]int, 0)
	for i, x := range a.nodes[v] {
		if n.size < x.size {
			best = append(best, i)
		}
	}
	if len(best) == 0 {
		for i, x := range a.nodes[v] {
			if n.size == x.size {
				best = append(best, i)
			}
		}
	}
	if len(best) == 0 {
		return false
	}
	idx := best[int(a.random.Next())%len(best)]
	a.onNodeDel(v, a.nodes[v][idx])
	a.onNodeAdd(v, n)
	a.nodes[v][idx] = n
	a.touch(v)
	return true
}
