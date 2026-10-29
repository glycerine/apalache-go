package lp

// Source: src/util/lp/lar_core_solver.h:276-352;
// src/util/lp/lp_core_solver_base.h:685-688.

// RemoveColumnFromInfSet erases before the source feasibility assertion.
func (m *PairCoreSolverBase) RemoveColumnFromInfSet(j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	m.InfSet.Erase(j)
	if sourceDebugEnabled {
		feasible, e := m.ColumnIsFeasible(j)
		if e != nil {
			return e
		}
		if !feasible {
			return ErrAssertion
		}
	}
	return nil
}

// UpdateXJAndGetDelta preserves source bound-selection and publication order.
// delta is caller-owned, including any partial assignment on error. In the
// source boxed-above-upper branch, X adds the upper bound instead of copying it.
func (c *LARCoreSolver) UpdateXJAndGetDelta(j uint32, position NonBasicColumnValuePosition, delta *RationalPair) (changed bool, err error) {
	if c.stopped || c.calls.pending() || c.signatureExpression != nil || c.signatureScalars != nil {
		return false, ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if c.RPrimal == nil || delta == nil || uint64(j) >= uint64(len(c.RX.Data)) {
		return false, ErrBounds
	}
	if err = c.RPrimal.ready(); err != nil {
		return false, err
	}
	before, e := c.calls.begin(c.manager)
	if e != nil {
		return false, e
	}
	changed, err = c.updateSignatureValue(j, position, delta)
	if err = c.calls.after(c.manager, before, err); err != nil {
		return false, err
	}
	// End this owner's numerical capture before delegating to the real core.
	if changed {
		err = c.RPrimal.RemoveColumnFromInfSet(j)
	}
	return changed, err
}

func (c *LARCoreSolver) updateSignatureValue(j uint32, position NonBasicColumnValuePosition, delta *RationalPair) (bool, error) {
	x := &c.RX.Data[j]
	var bound *RationalPair
	equalEarly, addUpper := false, false
	low := func() (*RationalPair, error) { return c.Bounds.Low.At(j) }
	upper := func() (*RationalPair, error) { return c.Bounds.Upper.At(j) }
	var err error
	switch position {
	case PositionAtLowBound:
		bound, err = low()
		equalEarly = true
	case PositionAtFixed, PositionAtUpperBound:
		bound, err = upper()
		equalEarly = true
	case PositionFreeOfBounds:
		return false, nil
	case PositionNotAtBound:
		kind, e := c.Bounds.Types.At(j)
		if e != nil {
			return false, e
		}
		switch kind {
		case ColumnFree:
			return false, nil
		case ColumnUpperBound:
			bound, err = upper()
		case ColumnLowBound, ColumnFixed:
			bound, err = low()
		case ColumnBoxed:
			bound, err = upper()
			if err != nil {
				return false, err
			}
			cmp, e := x.Compare(c.manager, bound)
			if e != nil {
				return false, e
			}
			if cmp > 0 {
				addUpper = true
			} else {
				bound, err = low()
			}
		default:
			if err = sourceAssertionFailure(); err != nil {
				return false, err
			}
		}
	default:
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	if err != nil {
		return false, err
	}
	if bound != nil {
		if equalEarly {
			same, e := x.Equal(c.manager, bound)
			if e != nil {
				return false, e
			}
			if same {
				return false, nil
			}
		}
		c.signatureExpression = NewIndexedPairVector(c.manager, 1)
		c.signatureScalars = NewIndexedRationalVector(c.manager, 3)
		tmp, px, py := &c.signatureScalars.Data[0], &c.signatureScalars.Data[1], &c.signatureScalars.Data[2]
		if err = etaRationalDifference(c.manager, tmp, px, &bound.X, &x.X); err != nil {
			return false, err
		}
		if err = etaRationalDifference(c.manager, tmp, py, &bound.Y, &x.Y); err != nil {
			return false, err
		}
		result := &c.signatureExpression.Data[0]
		if err = c.manager.Copy(&result.X, px); err != nil {
			return false, err
		}
		if err = c.manager.Copy(&result.Y, py); err != nil {
			return false, err
		}
		if err = delta.Copy(c.manager, result); err != nil {
			return false, err
		}
		if err = c.closeSignatureExpression(); err != nil {
			return false, err
		}
		if addUpper {
			err = x.Add(c.manager, x, bound)
		} else {
			err = x.Copy(c.manager, bound)
		}
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (c *LARCoreSolver) closeSignatureExpression() error {
	if err := c.signatureExpression.Close(); err != nil {
		return err
	}
	c.signatureExpression = nil
	if err := c.signatureScalars.Close(); err != nil {
		return err
	}
	c.signatureScalars = nil
	return nil
}

// updateSignatureBasic evaluates the owning pair product, subtracts it from
// the basic value, and destroys the expression before the core set update.
func (c *LARCoreSolver) updateSignatureBasic(link ColumnCell) (err error) {
	if uint64(link.Row) >= uint64(len(c.RBasis)) {
		return ErrBounds
	}
	basic := c.RBasis[link.Row]
	if uint64(basic) >= uint64(len(c.RX.Data)) {
		return ErrBounds
	}
	value, e := c.RA.ColumnValue(link)
	if e != nil {
		return e
	}
	c.signatureExpression = NewIndexedPairVector(c.manager, 1)
	c.signatureScalars = NewIndexedRationalVector(c.manager, 3)
	before, e := c.calls.begin(c.manager)
	if e != nil {
		return e
	}
	err = func() error {
		tmp, px, py := &c.signatureScalars.Data[0], &c.signatureScalars.Data[1], &c.signatureScalars.Data[2]
		delta := &c.signatureDelta.Data[0]
		if e := etaRationalProduct(c.manager, tmp, px, value, &delta.X); e != nil {
			return e
		}
		if e := etaRationalProduct(c.manager, tmp, py, value, &delta.Y); e != nil {
			return e
		}
		product := &c.signatureExpression.Data[0]
		if e := c.manager.Copy(&product.X, px); e != nil {
			return e
		}
		if e := c.manager.Copy(&product.Y, py); e != nil {
			return e
		}
		x := &c.RX.Data[basic]
		if e := x.Sub(c.manager, x, product); e != nil {
			return e
		}
		return c.closeSignatureExpression()
	}()
	if err = c.calls.after(c.manager, before, err); err != nil {
		return err
	}
	return c.RPrimal.UpdateColumnInInfSet(basic)
}

func (c *LARCoreSolver) PrepareSolverXWithSignatureTableau(signature LARSolutionSignature) (err error) {
	if c.stopped || c.calls.pending() || c.signatureDelta != nil || c.signatureExpression != nil || c.signatureScalars != nil {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	r := c.RPrimal
	if r == nil {
		return ErrBounds
	}
	if err = r.ready(); err != nil {
		return err
	}
	if sourceDebugEnabled {
		ok, e := r.InfSetIsCorrect()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	for _, j := range signatureKeys(signature) {
		if uint64(j) >= uint64(len(c.RHeading)) {
			return ErrBounds
		}
		if c.RHeading[j] >= 0 {
			continue
		}
		c.signatureDelta = NewIndexedPairVector(c.manager, 1)
		changed, e := c.UpdateXJAndGetDelta(j, signature[j], &c.signatureDelta.Data[0])
		if e != nil {
			return e
		}
		if changed {
			if uint64(j) >= uint64(len(c.RA.Columns)) {
				return ErrBounds
			}
			for _, link := range c.RA.Columns[j] {
				if err = c.updateSignatureBasic(link); err != nil {
					return err
				}
			}
			if sourceDebugEnabled {
				off, e := r.AMultXIsOff()
				if e != nil {
					return e
				}
				if off {
					return ErrAssertion
				}
			}
		}
		if err = c.signatureDelta.Close(); err != nil {
			return err
		}
		c.signatureDelta = nil
	}
	if sourceDebugEnabled {
		ok, e := r.InfSetIsCorrect()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	return nil
}
