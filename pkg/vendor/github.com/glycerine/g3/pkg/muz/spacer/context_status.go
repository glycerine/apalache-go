package spacer

// Bounded reports whether the latest search exhausted spacer.max_level.
// Source: context::solve_core sets datalog::BOUNDED at this exact exit.
func (c *Context) Bounded() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.bounded, nil
}
