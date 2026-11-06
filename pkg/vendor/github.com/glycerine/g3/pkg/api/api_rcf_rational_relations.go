package api

// Source: src/api/api_rcf.cpp:217-267.

func (c *SMTContext) rcfCompare(a, b *RCFNumber) (int, error) {
	service, err := c.checkRCF(a, b)
	if err != nil {
		return 0, err
	}
	if a.real != nil || b.real != nil {
		left, tl, e := c.realRCF(a)
		if e != nil {
			return 0, e
		}
		if tl {
			defer left.Close()
		}
		right, tr, e := c.realRCF(b)
		if e != nil {
			return 0, e
		}
		if tr {
			defer right.Close()
		}
		return c.realClosure.Compare(left, right)
	}
	less, err := service.Less(&a.value, &b.value)
	if err != nil || less {
		if less {
			return -1, err
		}
		return 0, err
	}
	greater, err := service.Greater(&a.value, &b.value)
	if greater {
		return 1, err
	}
	return 0, err
}

func (c *SMTContext) RCFLess(a, b *RCFNumber) (bool, error) {
	v, e := c.rcfCompare(a, b)
	return v < 0 && e == nil, e
}
func (c *SMTContext) RCFGreater(a, b *RCFNumber) (bool, error) {
	v, e := c.rcfCompare(a, b)
	return v > 0 && e == nil, e
}
func (c *SMTContext) RCFLessEqual(a, b *RCFNumber) (bool, error) {
	v, e := c.rcfCompare(a, b)
	return v <= 0 && e == nil, e
}
func (c *SMTContext) RCFGreaterEqual(a, b *RCFNumber) (bool, error) {
	v, e := c.rcfCompare(a, b)
	return v >= 0 && e == nil, e
}
func (c *SMTContext) RCFEqual(a, b *RCFNumber) (bool, error) {
	v, e := c.rcfCompare(a, b)
	return v == 0 && e == nil, e
}
func (c *SMTContext) RCFNotEqual(a, b *RCFNumber) (bool, error) {
	v, e := c.rcfCompare(a, b)
	return v != 0 && e == nil, e
}
