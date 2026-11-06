package qe

func (c *arithmeticCompleteProject) selectBranch() (uint32, error) {
	integer, e := c.plugin.util.isInteger(c.target)
	if e != nil {
		return 0, e
	}
	if integer {
		return c.call.integerBranch(c.entry, c.target)
	}
	return c.call.realBranch(c.entry, c.target)
}
