package arith

// Source: src/tactic/arith/fm_tactic.cpp:322-371.
type FMConstraintSet struct {
	positions []int
	values    []*FMConstraint
}

func (s *FMConstraintSet) Contains(c *FMConstraint) bool {
	return c != nil && int(c.ID) < len(s.positions) && s.positions[c.ID] >= 0
}
func (s *FMConstraintSet) Len() int { return len(s.values) }
func (s *FMConstraintSet) Insert(c *FMConstraint) {
	for len(s.positions) <= int(c.ID) {
		s.positions = append(s.positions, -1)
	}
	if s.positions[c.ID] >= 0 {
		return
	}
	s.positions[c.ID] = len(s.values)
	s.values = append(s.values, c)
}
func (s *FMConstraintSet) Erase(c *FMConstraint) {
	if !s.Contains(c) {
		return
	}
	p := s.positions[c.ID]
	s.positions[c.ID] = -1
	last := len(s.values) - 1
	if p != last {
		s.values[p] = s.values[last]
		s.positions[s.values[p].ID] = p
	}
	s.values = s.values[:last]
}
func (s *FMConstraintSet) Pop() *FMConstraint {
	if len(s.values) == 0 {
		return nil
	}
	c := s.values[len(s.values)-1]
	s.positions[c.ID] = -1
	s.values = s.values[:len(s.values)-1]
	return c
}
func (s *FMConstraintSet) Reset() { s.positions = nil; s.values = nil }
