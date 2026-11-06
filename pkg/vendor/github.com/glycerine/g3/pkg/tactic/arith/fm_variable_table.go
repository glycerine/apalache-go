package arith

// Source: src/tactic/arith/fm_tactic.cpp:864-946.
type FMVariableTable struct {
	Integer   []bool
	Forbidden []bool
	Index     *FMConstraintIndex
}

func NewFMVariableTable() *FMVariableTable {
	return &FMVariableTable{Index: NewFMConstraintIndex()}
}

func (t *FMVariableTable) Add(integer, forbidden bool) uint32 {
	v := uint32(len(t.Integer))
	t.Integer = append(t.Integer, integer)
	t.Forbidden = append(t.Forbidden, forbidden)
	if forbidden {
		t.Index.Forbidden[v] = true
	}
	return v
}

func (t *FMVariableTable) IsInteger(v uint32) bool {
	return int(v) < len(t.Integer) && t.Integer[v]
}

func (t *FMVariableTable) IsForbidden(v uint32) bool {
	return int(v) >= len(t.Forbidden) || t.Forbidden[v]
}

func (t *FMVariableTable) AllInteger(c *FMConstraint) bool {
	for _, v := range c.Variables {
		if !t.IsInteger(v) {
			return false
		}
	}
	return true
}
