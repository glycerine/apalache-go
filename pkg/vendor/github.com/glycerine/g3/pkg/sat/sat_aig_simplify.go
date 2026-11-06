package sat

func (a *satAIGCuts) Simplify() error {
	if err := a.open(); err != nil {
		return err
	}
	for _, cs := range a.cuts {
		for i := 0; i < len(cs.cuts); i++ {
			c := cs.cuts[i]
			table := c.Table()
			for j := uint32(0); j < c.size && j < 6; j++ {
				diff := satCutEffectMask(j) & (table ^ (table >> (uint64(1) << j)))
				if diff == 0 {
					d := c
					d.RemoveElem(j)
					if _, e := cs.Insert(a.onCutAdd, a.onCutDel, d); e != nil {
						return e
					}
					_, e := cs.Evict(a.onCutDel, c)
					if e != nil {
						return e
					}
					i = -1
					break
				}
			}
		}
	}
	return nil
}
