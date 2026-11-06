package sat

func (a *satAIGCuts) Compute() ([]*satCutSet, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	if a.config.full {
		if err := a.flushRoots(); err != nil {
			return nil, err
		}
	}
	for v, ns := range a.nodes {
		for _, n := range ns {
			if err := a.augment(uint32(v), n); err != nil {
				return nil, err
			}
		}
	}
	a.numCutCalls++
	return a.cuts, nil
}
func (a *satAIGCuts) augment(v uint32, n satAIGNode) error {
	a.insertions = 0
	if !a.nodeTouched(v, n) {
		return nil
	}
	cs := a.cuts[v]
	var err error
	switch {
	case n.isVar():
		return nil
	case n.isLUT():
		err = a.augmentLUT(v, n, cs)
	case n.isITE():
		err = a.augmentITE(v, n, cs)
	case n.size == 0:
		err = a.augmentAIG0(v, n, cs)
	case n.size == 1:
		err = a.augmentAIG1(v, n, cs)
	case n.size == 2:
		err = a.augmentAIG2(v, n, cs)
	case n.size <= satCutMaxSize:
		err = a.augmentAIGN(v, n, cs)
	}
	if a.insertions > 0 {
		a.touch(v)
	}
	return err
}
func (a *satAIGCuts) nodeTouched(v uint32, n satAIGNode) bool {
	for i := uint32(0); i < n.size; i++ {
		if a.touched(uint32(a.child(n, i).variable())) {
			return true
		}
	}
	return a.touched(v)
}
func (a *satAIGCuts) insertCut(v uint32, c satCut, cs *satCutSet) (bool, error) {
	ok, err := cs.Insert(a.onCutAdd, a.onCutDel, c)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	a.numCuts++
	a.insertions++
	max := a.MaxCutSetSize(v)
	if a.insertions > max {
		return false, nil
	}
	for cs.Size() >= max && cs.Size() > 1 {
		idx := uint32(1) + a.random.Next()%(cs.Size()-1)
		if err := cs.EvictIndex(a.onCutDel, idx); err != nil {
			return false, err
		}
	}
	return true, nil
}
