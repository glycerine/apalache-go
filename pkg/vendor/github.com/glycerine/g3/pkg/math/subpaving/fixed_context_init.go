package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1753-1808,
// concrete config_mpfx root initialization order.

// InitializeRoot asserts units, propagates definitions in variable order and
// drains the resulting queue. A canceled source initialization is not resumed
// because the root remains allocated.
func (c *FixedContext) InitializeRoot() error {
	if err := c.open(); err != nil {
		return err
	}
	n := c.root
	if c.rootInitStarted {
		return nil
	}
	if err := n.open(); err != nil {
		return err
	}
	if c.initStage == 0 {
		// Source publishes m_root before displaying the initial constraints.
		c.rootInitStarted = true
		if !c.initConstraintsTraced {
			if err := c.traceInitialConstraints(); err != nil {
				return err
			}
			c.initConstraintsTraced = true
		}
		if err := c.AssertUnitClauses(n); err != nil {
			return err
		}
		c.initStage = 1
	}
	if c.initStage == 1 {
		for c.initDefHead < uint32(len(c.isInt)) && !n.inconsistent {
			variable := c.initDefHead
			if _, ok := c.definitionVisited(variable); ok {
				if err := c.propagateDefinition(variable, n); err != nil {
					return err
				}
			}
			c.initDefHead++
		}
		c.initStage = 2
	}
	if c.initStage == 2 {
		if err := c.PropagateQueued(n); err != nil {
			return err
		}
		c.initStage = 3
	}
	if c.initStage == 3 && !c.initRootBoundsTraced {
		if err := c.traceInitialBounds(n, "root bounds after propagation\n", 1801); err != nil {
			return err
		}
		c.initRootBoundsTraced = true
	}
	return nil
}
