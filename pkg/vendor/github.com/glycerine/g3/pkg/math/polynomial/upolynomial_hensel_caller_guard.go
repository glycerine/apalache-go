package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:703-801,873-977;
// src/util/mpzzp.h:97-105.

// The synchronous chain is an entry/destructor guard, never acquisition
// authority. Explicit concrete contexts alone stamp actual numerical leaves.
func (c *upolyHenselCallerScope) enterCall() {
	if c.callDepth == 0 {
		for i, m := range c.numericOwners() {
			if m != nil {
				duplicate := false
				for j := 0; j < i; j++ {
					if c.callOwners[j] == m {
						duplicate = true
						break
					}
				}
				if duplicate {
					continue
				}
				c.callOwners[i] = m
				c.callerPrevious[i] = m.henselCallerGuard
				c.callerPreviousInvocation[i] = m.henselCallerInvocation
				m.henselCallerGuard = c
				m.henselCallerInvocation = c.invocation
			}
		}
	}
	c.callDepth++
}
func (c *upolyHenselCallerScope) attachRelated() {
	if c.callDepth != 0 && c.related != nil && c.callOwners[1] == nil {
		m := c.related
		c.callOwners[1] = m
		c.callerPrevious[1] = m.henselCallerGuard
		c.callerPreviousInvocation[1] = m.henselCallerInvocation
		m.henselCallerGuard = c
		m.henselCallerInvocation = c.invocation
	}
}
func (c *upolyHenselCallerScope) leaveCall() {
	c.callDepth--
	if c.callDepth == 0 {
		for i, m := range c.callOwners {
			if m != nil {
				m.henselCallerGuard = c.callerPrevious[i]
				m.henselCallerInvocation = c.callerPreviousInvocation[i]
			}
		}
		c.callOwners = [4]*UPolynomialManager{}
		c.callerPrevious = [4]*upolyHenselCallerScope{}
		c.callerPreviousInvocation = [4]uint64{}
	}
}
func (m *UPolynomialManager) henselCallerDestructorAuthorized(c *upolyHenselCallerScope, own bool) bool {
	if c == nil || !c.matches() || m != c.related || m.henselCallerGuard != c || m.henselCallerInvocation != c.invocation || c.callDepth == 0 || m.callbackDepth != 0 {
		return false
	}
	// A public nil entry cannot acquire this private related destructor permit.
	if own {
		return c.ownExecuting
	}
	return c.sourceExecuting || c.ownExecuting
}

// Detach only after the actual related owner consumed its cold storage.
func (c *upolyHenselCallerScope) detachRelated() {
	if m := c.callOwners[1]; m != nil {
		m.henselCallerGuard = c.callerPrevious[1]
		m.henselCallerInvocation = c.callerPreviousInvocation[1]
		c.callOwners[1] = nil
		c.callerPrevious[1] = nil
		c.callerPreviousInvocation[1] = 0
	}
}
