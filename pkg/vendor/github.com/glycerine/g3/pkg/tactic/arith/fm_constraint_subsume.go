package arith

// Source: src/tactic/arith/fm_tactic.cpp:626-718.
func FMConstraintSubsumes(a, b *FMConstraint) bool {
	return fmConstraintSubsumes(a, b, nil)
}

func fmConstraintSubsumes(a, b *FMConstraint, counter *uint32) bool {
	if a == nil || b == nil || a == b || len(a.Literals) > len(b.Literals) || len(a.Variables) != len(b.Variables) || a.Constant.Cmp(&b.Constant) > 0 {
		return false
	}
	if !a.Strict && b.Strict && a.Constant.Cmp(&b.Constant) == 0 {
		return false
	}
	if counter != nil {
		*counter += uint32(len(a.Literals) + len(b.Literals))
	}
	pos := make(map[uint32]int, len(a.Variables))
	for i, v := range a.Variables {
		pos[v] = i
	}
	for i, v := range b.Variables {
		j, ok := pos[v]
		if !ok || a.Coefficients[j].Cmp(&b.Coefficients[i]) != 0 {
			return false
		}
	}
	signs := make(map[uint32]int8, len(b.Literals))
	for _, l := range b.Literals {
		v := uint32(l)
		s := int8(1)
		if l < 0 {
			v = uint32(-l)
			s = -1
		}
		signs[v] = s
	}
	for _, l := range a.Literals {
		v := uint32(l)
		s := int8(1)
		if l < 0 {
			v = uint32(-l)
			s = -1
		}
		if signs[v] != s {
			return false
		}
	}
	return true
}
