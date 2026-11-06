package sat

func (a *satAIGCuts) augmentAIG0(_ uint32, n satAIGNode, cs *satCutSet) error {
	if err := cs.Reset(a.onCutDel); err != nil {
		return err
	}
	var c satCut
	if n.sign {
		c.SetTable(0)
	} else {
		c.SetTable(1)
	}
	return cs.PushBack(a.onCutAdd, c)
}
func (a *satAIGCuts) augmentAIG1(v uint32, n satAIGNode, cs *satCutSet) error {
	lit := a.child(n, 0)
	src := a.cuts[lit.variable()]
	for _, x := range src.cuts {
		c := x
		if n.sign {
			c.Negate()
		}
		ok, e := a.insertCut(v, c, cs)
		if e != nil {
			return e
		}
		if !ok {
			return nil
		}
	}
	return nil
}
func (a *satAIGCuts) augmentAIG2(v uint32, n satAIGNode, cs *satCutSet) error {
	l1, l2 := a.child(n, 0), a.child(n, 1)
	for _, x := range a.cuts[l1.variable()].cuts {
		for _, y := range a.cuts[l2.variable()].cuts {
			var c satCut
			if !c.Merge(x, y) {
				continue
			}
			t1, t2 := x.ShiftTable(c), y.ShiftTable(c)
			if l1.sign() {
				t1 = ^t1
			}
			if l2.sign() {
				t2 = ^t2
			}
			if n.isAnd() {
				c.SetTable(t1 & t2)
			} else {
				c.SetTable(t1 ^ t2)
			}
			if n.sign {
				c.Negate()
			}
			ok, e := a.insertCut(v, c, cs)
			if e != nil {
				return e
			}
			if !ok {
				return nil
			}
		}
	}
	return nil
}
func (a *satAIGCuts) augmentITE(v uint32, n satAIGNode, cs *satCutSet) error {
	l1, l2, l3 := a.child(n, 0), a.child(n, 1), a.child(n, 2)
	for _, x := range a.cuts[l1.variable()].cuts {
		for _, y := range a.cuts[l2.variable()].cuts {
			var xy satCut
			if !xy.Merge(x, y) {
				continue
			}
			for _, z := range a.cuts[l3.variable()].cuts {
				var c satCut
				if !c.Merge(xy, z) {
					continue
				}
				t1, t2, t3 := x.ShiftTable(c), y.ShiftTable(c), z.ShiftTable(c)
				if l1.sign() {
					t1 = ^t1
				}
				if l2.sign() {
					t2 = ^t2
				}
				if l3.sign() {
					t3 = ^t3
				}
				c.SetTable(t1&t2 | ^t1&t3)
				if n.sign {
					c.Negate()
				}
				ok, e := a.insertCut(v, c, cs)
				if e != nil {
					return e
				}
				if !ok {
					return nil
				}
			}
		}
	}
	return nil
}
