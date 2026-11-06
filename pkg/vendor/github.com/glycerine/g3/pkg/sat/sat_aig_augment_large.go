package sat

func (a *satAIGCuts) augmentAIGN(v uint32, n satAIGNode, cs *satCutSet) error {
	left, e := newSATCutSet(a.budget)
	if e != nil {
		return e
	}
	defer left.Close()
	right, e := newSATCutSet(a.budget)
	if e != nil {
		return e
	}
	defer right.Close()
	left.Init(a.config.maxCutSetSize+1, ^uint32(0))
	right.Init(a.config.maxCutSetSize+1, ^uint32(0))
	lit := a.child(n, 0)
	for _, x := range a.cuts[lit.variable()].cuts {
		c := x
		if lit.sign() {
			c.Negate()
		}
		if e = left.PushBack(a.onCutAdd, c); e != nil {
			return e
		}
	}
	for i := uint32(1); i < n.size; i++ {
		if e = right.Reset(a.onCutDel); e != nil {
			return e
		}
		lit = a.child(n, i)
		a.insertions = 0
		stop := false
		for _, x := range left.cuts {
			for _, y := range a.cuts[lit.variable()].cuts {
				var c satCut
				if !c.Merge(x, y) {
					continue
				}
				t1, t2 := x.ShiftTable(c), y.ShiftTable(c)
				if lit.sign() {
					t2 = ^t2
				}
				if n.isAnd() {
					c.SetTable(t1 & t2)
				} else {
					c.SetTable(t1 ^ t2)
				}
				if i+1 == n.size && n.sign {
					c.Negate()
				}
				ok, er := a.insertCut(^uint32(0), c, right)
				if er != nil {
					return er
				}
				if !ok {
					stop = true
					break
				}
			}
			if stop {
				break
			}
		}
		if e = left.Swap(right); e != nil {
			return e
		}
	}
	a.insertions = 0
	for _, c := range left.cuts {
		ok, er := a.insertCut(v, c, cs)
		if er != nil {
			return er
		}
		if !ok {
			break
		}
	}
	return nil
}
func (a *satAIGCuts) augmentLUT(v uint32, n satAIGNode, cs *satCutSet) error {
	if n.size == 0 {
		var c satCut
		c.SetTable(n.lut)
		_, e := a.insertCut(v, c, cs)
		return e
	}
	tables := make([]satCut, n.size)
	return a.augmentLUTRec(v, n, cs, tables, 0, satCut{})
}
func (a *satAIGCuts) augmentLUTRec(v uint32, n satAIGNode, cs *satCutSet, tables []satCut, idx uint32, merged satCut) error {
	if idx < n.size {
		lit := a.child(n, idx)
		for _, x := range a.cuts[lit.variable()].cuts {
			next := x
			if idx > 0 {
				next = satCut{}
				if !next.Merge(merged, x) {
					continue
				}
			}
			tables[idx] = x
			if e := a.augmentLUTRec(v, n, cs, tables, idx+1, next); e != nil {
				return e
			}
		}
		return nil
	}
	luts := make([]uint64, n.size)
	for i := range luts {
		luts[i] = tables[i].ShiftTable(merged)
	}
	var table uint64
	for assignment := uint32(0); assignment < uint32(1)<<merged.size; assignment++ {
		var w uint32
		for i := uint32(0); i < n.size; i++ {
			bit := (luts[i] >> assignment) & 1
			if a.child(n, i).sign() {
				bit ^= 1
			}
			w |= uint32(bit) << i
		}
		table |= (n.lut >> w) & 1 << assignment
	}
	merged.SetTable(table)
	_, e := a.insertCut(v, merged, cs)
	return e
}
