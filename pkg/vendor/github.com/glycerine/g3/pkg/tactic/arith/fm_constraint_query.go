package arith

func (c *FMConstraint) Live() bool { return c != nil && !c.Dead }
func (c *FMConstraint) VariablePosition(v uint32) (int, bool) {
	for i, x := range c.Variables {
		if x == v {
			return i, true
		}
	}
	return 0, false
}
