package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1773-1808, concrete config_mpq
// root creation, unit assertion, definition sweep and bound propagation.

import "math"

// InitializeRoot follows context_t::init; after a stopped source checkpoint,
// the existing root prevents a later call from resuming initialization.
func (c *RationalContext) InitializeRoot() error {
	if err := c.open(); err != nil {
		return err
	}
	n, err := c.Root()
	if err != nil {
		return err
	}
	if c.rootInitStarted {
		return nil
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
		if _, err := c.AssertRootUnits(); err != nil {
			return err
		}
		c.initStage = 1
	}
	if c.initStage == 1 {
		for c.initDefHead < uint32(len(c.variables)) && n.conflictVar == math.MaxUint32 {
			variable := c.initDefHead
			if c.polynomials[variable] != nil {
				if err := c.PropagatePolynomial(variable, n); err != nil {
					return err
				}
			}
			if c.monomials[variable] != nil && n.conflictVar == math.MaxUint32 {
				if err := c.PropagateMonomial(variable, n); err != nil {
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
