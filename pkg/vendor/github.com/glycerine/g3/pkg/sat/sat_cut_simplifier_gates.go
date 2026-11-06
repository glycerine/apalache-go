package sat

func (c *satCutSimplifierLive) AddAnd(h Literal, args []Literal) error {
	e := c.aig.AddNode(h, satAIGAnd, args)
	if e == nil {
		c.stats.numAnds++
	}
	return e
}
func (c *satCutSimplifierLive) AddOr(h Literal, args []Literal) error {
	x := make([]Literal, len(args))
	for i, l := range args {
		x[i] = l.Negated()
	}
	e := c.aig.AddNode(h.Negated(), satAIGAnd, x)
	if e == nil {
		c.stats.numAnds++
	}
	return e
}
func (c *satCutSimplifierLive) AddXor(h Literal, args []Literal) error {
	e := c.aig.AddNode(h, satAIGXor, args)
	if e == nil {
		c.stats.numXors++
	}
	return e
}
func (c *satCutSimplifierLive) AddITE(h, cond, yes, no Literal) error {
	e := c.aig.AddNode(h, satAIGITE, []Literal{cond, yes, no})
	if e == nil {
		c.stats.numITEs++
	}
	return e
}
func (c *satCutSimplifierLive) AddIFF(h, a, b Literal) error {
	return c.AddXor(h, []Literal{a, b.Negated()})
}
func (c *satCutSimplifierLive) SetRoot(v BoolVar, r Literal) error { return c.aig.SetRoot(v, r) }
