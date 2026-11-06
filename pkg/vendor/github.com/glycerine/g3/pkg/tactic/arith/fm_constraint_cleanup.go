package arith

// Source: src/tactic/arith/fm_tactic.cpp:1195-1209,1260-1268.
func fmLiveConstraints(cs []*FMConstraint) []*FMConstraint {
	out := cs[:0]
	for _, c := range cs {
		if c != nil && !c.Dead {
			out = append(out, c)
		}
	}
	return out
}

func (t *FMVariableTable) Cleanup(x uint32) {
	t.Index.Lower[x] = fmLiveConstraints(t.Index.Lower[x])
	t.Index.Upper[x] = fmLiveConstraints(t.Index.Upper[x])
}

func (t *FMVariableTable) LiveBounds(x uint32, lower bool) []*FMConstraint {
	cs := t.Index.Upper[x]
	if lower {
		cs = t.Index.Lower[x]
	}
	out := make([]*FMConstraint, 0, len(cs))
	for _, c := range cs {
		if c != nil && !c.Dead {
			out = append(out, c)
		}
	}
	return out
}
