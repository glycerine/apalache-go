package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1384-1474.

func (c *simpleSumChecker) evaluateGeneral(form *simpleGeneralForm, domains []simplePowerDomain) (possible bool, err error) {
	if form.literal.Sign() && form.atom.kind == AtomEQ {
		return true, nil
	}
	actual := simpleEQ
	for i := uint32(0); i < form.atom.Size(); i++ {
		p, cause := form.atom.Polynomial(i)
		if cause != nil {
			return false, cause
		}
		interval, cause := c.polynomialInterval(p, domains)
		if cause != nil {
			return false, cause
		}
		sign, cause := classifySimpleInterval(c.solver.config.Algebraic, &interval)
		closeCause := interval.close(c.solver.config.Algebraic)
		if cause == nil {
			cause = closeCause
		}
		if cause != nil {
			return false, cause
		}
		even, cause := form.atom.IsEven(i)
		if cause != nil {
			return false, cause
		}
		sign = paritySimpleSign(sign, even)
		if i == 0 {
			actual = sign
		} else {
			actual = multiplySimpleSign(actual, sign)
		}
		if actual == simpleNone {
			return true, nil
		}
	}
	return simpleSignsCompatible(actual, form.sign), nil
}
