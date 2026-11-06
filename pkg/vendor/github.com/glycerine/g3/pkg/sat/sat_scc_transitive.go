package sat

// Source: src/sat/sat_scc.cpp:225-250; src/sat/sat_scc.h:37-66.

func (c *satSCC) initBIG(learned bool) error {
	if c.big == nil {
		if c.solver != nil {
			c.big = newSATBIG(&c.solver.decision.rng)
		} else {
			c.big = newSATBIG(nil)
		}
	}
	return c.big.initLifecycle(c.lifecycle, learned)
}

func (c *satSCC) ensureBIG(learned bool) error {
	if c.big == nil || len(c.big.left) == 0 || c.big.learned != learned {
		return c.initBIG(learned)
	}
	return nil
}

func (c *satSCC) reduceTransitiveOnce(learned bool) (uint32, error) {
	if err := c.initBIG(learned); err != nil {
		return 0, err
	}
	n, err := c.big.reduceTransitive(c.lifecycle)
	c.numElimBinary += n
	return n, err
}

func (c *satSCC) reduceTransitive() error {
	for _, learned := range []bool{false, true} {
		var quota uint32
		for count := uint32(0); count <= 10; count++ {
			n, err := c.reduceTransitiveOnce(learned)
			if err != nil {
				return err
			}
			if n <= quota {
				break
			}
			quota = n / 2
			if quota < 100 {
				quota = 100
			}
		}
	}
	return nil
}

func (c *satSCC) Connected(u, v Literal) bool {
	return c.big != nil && c.big.connected(u, v)
}

func (c *satSCC) bigLeft(l Literal) int32 {
	if c.big == nil {
		return 0
	}
	return c.big.getLeft(l)
}

func (c *satSCC) bigRight(l Literal) int32 {
	if c.big == nil {
		return 0
	}
	return c.big.getRight(l)
}
