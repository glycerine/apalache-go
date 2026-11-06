package arith

// Source: src/tactic/arith/fm_tactic.cpp:1211-1258.
func (t *FMVariableTable) analyze(c *FMConstraint, x uint32) (allInteger, unitCoefficient bool) {
	allInteger, unitCoefficient = true, true
	for i, v := range c.Variables {
		if !t.IsInteger(v) {
			return false, false
		}
		if v == x {
			a := &c.Coefficients[i]
			unitCoefficient = a.IsInt() && a.Num().IsInt64() && (a.Num().Int64() == 1 || a.Num().Int64() == -1)
		}
	}
	return allInteger, unitCoefficient
}

func (t *FMVariableTable) analyzeList(cs []*FMConstraint, x uint32) (allInteger, unitCoefficient bool) {
	allInteger, unitCoefficient = true, true
	for _, c := range cs {
		if c.Dead {
			continue
		}
		all, unit := t.analyze(c, x)
		if !all {
			return false, false
		}
		if !unit {
			unitCoefficient = false
		}
	}
	return allInteger, unitCoefficient
}

func (t *FMVariableTable) CanEliminate(x uint32) bool {
	if t.IsForbidden(x) {
		return false
	}
	if !t.IsInteger(x) {
		return true
	}
	all, lowerUnit := t.analyzeList(t.Index.Lower[x], x)
	if !all {
		return false
	}
	all, upperUnit := t.analyzeList(t.Index.Upper[x], x)
	return all && (lowerUnit || upperUnit)
}
