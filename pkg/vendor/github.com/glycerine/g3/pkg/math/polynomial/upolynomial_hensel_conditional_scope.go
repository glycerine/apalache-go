package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:805-871;
// src/util/mpz.cpp:1453-1507.

func (c *upolyHenselCallerScope) finishIndividualSource() error {
	c.enterCall()
	defer c.leaveCall()
	previous := c.sourceExecuting
	c.sourceExecuting = true
	defer func() { c.sourceExecuting = previous }()
	if c.sourceCause != nil {
		return c.sourceCause
	}
	if e := c.numericSourceCause(); e != nil {
		c.sourceCause = e
		return e
	}
	return c.finishVectors(c.individual[:], &c.individualSource)
}
func (c *upolyHenselCallerScope) finishConditionalSource() error {
	c.enterCall()
	defer c.leaveCall()
	previous := c.sourceExecuting
	c.sourceExecuting = true
	defer func() { c.sourceExecuting = previous }()
	if e := c.owner.traceSourceBlocker(); e != nil {
		c.sourceCause = e
		return e
	}
	if e := c.finishIndividualSource(); e != nil {
		return e
	}
	if e := c.finishVectors(c.checkVectors[:], &c.checkSource); e != nil {
		return e
	}
	if c.checkPowerConstructed && !c.checkPowerEnded {
		if _, e := upolynomialSourceNumeralDel(c.owner.manager.config.Integer, &c.checkPower); e != nil {
			c.sourceCause = e
			return e
		}
		c.checkPowerEnded = true
	}
	return nil
}
func (c *upolyHenselCallerScope) ownConditional() error {
	if e := c.cleanupNumeric(false); e != nil {
		return e
	}
	if e := c.ownVectors(c.individual[:], &c.individualOwn); e != nil {
		return e
	}
	if e := c.ownVectors(c.checkVectors[:], &c.checkOwn); e != nil {
		return e
	}
	if c.checkPowerConstructed && !c.checkPowerEnded && !c.checkPowerOwned {
		if e := c.owner.manager.config.Integer.CleanupRawNumeral(&c.checkPower); e != nil {
			return e
		}
		c.checkPowerOwned = true
	}
	return nil
}
