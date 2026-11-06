package arith

// Source: src/tactic/arith/fm_tactic.cpp:720-775.
type FMConstraintIndex struct {
	diagnostics  *fmConstraintDiagnostics
	Counter      *uint32
	Lower, Upper map[uint32][]*FMConstraint
	Forbidden    map[uint32]bool
	Todo         FMConstraintSet
}

func NewFMConstraintIndex() *FMConstraintIndex {
	return &FMConstraintIndex{Lower: map[uint32][]*FMConstraint{}, Upper: map[uint32][]*FMConstraint{}, Forbidden: map[uint32]bool{}}
}
func (i *FMConstraintIndex) Add(c *FMConstraint) {
	for n, v := range c.Variables {
		if i.Forbidden[v] {
			continue
		}
		if c.Coefficients[n].Sign() < 0 {
			i.Lower[v] = append(i.Lower[v], c)
		} else {
			i.Upper[v] = append(i.Upper[v], c)
		}
	}
	i.Todo.Insert(c)
}
func (i *FMConstraintIndex) Backward(c *FMConstraint) error {
	var candidates []*FMConstraint
	var bestVariable uint32
	bestLower := false
	best := -1
	for n, v := range c.Variables {
		if i.Forbidden[v] {
			continue
		}
		xs := i.Upper[v]
		if c.Coefficients[n].Sign() < 0 {
			xs = i.Lower[v]
		}
		if best < 0 || len(xs) < best {
			best = len(xs)
			candidates = xs
			bestVariable = v
			bestLower = c.Coefficients[n].Sign() < 0
		}
	}
	kept := candidates[:0]
	if i.Counter != nil && best > 0 {
		*i.Counter += uint32(best)
	}
	for _, other := range candidates {
		if !other.Dead && fmConstraintSubsumes(c, other, i.Counter) {
			if err := i.diagnostics.subsumed(c, other); err != nil {
				return err
			}
			other.Dead = true
		}
		if !other.Dead {
			kept = append(kept, other)
		}
	}
	if best >= 0 {
		if bestLower {
			i.Lower[bestVariable] = kept
		} else {
			i.Upper[bestVariable] = kept
		}
	}
	return nil
}
func (i *FMConstraintIndex) Subsume() error {
	for i.Todo.Len() != 0 {
		c := i.Todo.Pop()
		if !c.Dead {
			if err := i.Backward(c); err != nil {
				return err
			}
		}
	}
	return nil
}
