package nlsat

import "errors"

func (c *simpleSumChecker) possible(candidate *simpleSumCandidate) (possible bool, err error) {
	am := c.solver.config.Algebraic
	copies, err := cloneSimplePowerDomains(am, c.domains)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, closeSimplePowerDomains(am, copies)) }()
	_, possible, err = c.apply(candidate, copies)
	return
}
func (c *simpleSumChecker) force(candidate *simpleSumCandidate) (bool, error) {
	changed, ok, err := c.apply(candidate, c.domains)
	c.improved = c.improved || changed
	return ok, err
}
func (c *simpleSumChecker) apply(candidate *simpleSumCandidate, domains []simplePowerDomain) (changed, possible bool, err error) {
	am := c.solver.config.Algebraic
	switch candidate.kind {
	case 1:
		f := &candidate.linear
		if uint64(f.variable) >= uint64(len(domains)) {
			return false, true, nil
		}
		changed, possible, err = domains[f.variable].original.intersect(am, f.sign, &f.boundary)
		if err == nil && possible {
			var ok bool
			ok, err = domains[f.variable].refreshMagnitude(am)
			possible = possible && ok
		}
		return
	case 2:
		f := &candidate.power
		if uint64(f.variable) >= uint64(len(domains)) {
			return false, true, nil
		}
		return domains[f.variable].apply(am, f)
	case 3:
		return c.evaluateSum(&candidate.sum, domains)
	}
	return false, true, nil
}
