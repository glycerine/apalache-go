package lp

// Source: src/util/lp/lp_primal_core_solver.h:329,368;
// src/util/lp/lp_settings.h:368 and numeric_pair.h:142-164,192-194,231-233.

// ZeroHarrisEps assigns the precise scalar zero without replacing its owner.
func (m *PairPrimalCoreSolver) ZeroHarrisEps() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.manager.SetInt32(&m.ConvertedHarrisEpsilon, 0)
}

// HarrisEpsForBound evaluates the full source expression before assigning out.
// out may alias bound; all inputs are read before output assignment.
// On failure the primal retains every expression value for owning Close.
func (m *PairPrimalCoreSolver) HarrisEpsForBound(bound, out *RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if bound == nil || out == nil {
		return ErrBounds
	}
	return m.harrisEpsForBoundValue(bound, out)
}

// The same primal owner evaluates this while its bound-step epsilon is live.
func (m *PairPrimalCoreSolver) harrisEpsForBoundValue(bound, out *RationalPair) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 6)
	m.pairWork = NewIndexedPairVector(m.manager, 5)
	tmp, qx, qy := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	one, ten, three := &m.work.Data[3], &m.work.Data[4], &m.work.Data[5]
	if err = m.manager.SetInt32(one, 1); err != nil {
		return err
	}
	if err = m.manager.SetInt32(ten, 10); err != nil {
		return err
	}
	if err = m.manager.SetInt32(three, 3); err != nil {
		return err
	}
	absolute, divided, shifted, scaled, result := &m.pairWork.Data[0], &m.pairWork.Data[1], &m.pairWork.Data[2], &m.pairWork.Data[3], &m.pairWork.Data[4]
	zero := NewRationalPair()
	cmp, e := bound.Compare(m.manager, &zero)
	if e != nil {
		return e
	}
	if cmp < 0 {
		// Unary pair minus constructs both scalar results before copying them
		// into the returned pair's fields.
		if err = m.manager.Copy(qx, &bound.X); err != nil {
			return err
		}
		if err = m.manager.Neg(qx); err != nil {
			return err
		}
		if err = m.manager.Copy(qy, &bound.Y); err != nil {
			return err
		}
		if err = m.manager.Neg(qy); err != nil {
			return err
		}
		if err = m.manager.Copy(&absolute.X, qx); err != nil {
			return err
		}
		if err = m.manager.Copy(&absolute.Y, qy); err != nil {
			return err
		}
	} else if err = absolute.Copy(m.manager, bound); err != nil {
		return err
	}
	if err = etaRationalQuotient(m.manager, tmp, qx, &absolute.X, ten); err != nil {
		return err
	}
	if err = etaRationalQuotient(m.manager, tmp, qy, &absolute.Y, ten); err != nil {
		return err
	}
	if err = m.manager.Copy(&divided.X, qx); err != nil {
		return err
	}
	if err = m.manager.Copy(&divided.Y, qy); err != nil {
		return err
	}
	// numeric_pair::operator+ evaluates a.x + x, not x + a.x.
	if err = m.manager.Copy(tmp, &divided.X); err != nil {
		return err
	}
	if err = m.manager.Add(tmp, one, tmp); err != nil {
		return err
	}
	if err = m.manager.Copy(qx, tmp); err != nil {
		return err
	}
	if err = m.manager.Del(tmp); err != nil {
		return err
	}
	if err = m.manager.Copy(tmp, &divided.Y); err != nil {
		return err
	}
	if err = m.manager.Add(tmp, &zero.Y, tmp); err != nil {
		return err
	}
	if err = m.manager.Copy(qy, tmp); err != nil {
		return err
	}
	if err = m.manager.Del(tmp); err != nil {
		return err
	}
	if err = m.manager.Copy(&shifted.X, qx); err != nil {
		return err
	}
	if err = m.manager.Copy(&shifted.Y, qy); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, qx, &m.ConvertedHarrisEpsilon, &shifted.X); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, qy, &m.ConvertedHarrisEpsilon, &shifted.Y); err != nil {
		return err
	}
	if err = m.manager.Copy(&scaled.X, qx); err != nil {
		return err
	}
	if err = m.manager.Copy(&scaled.Y, qy); err != nil {
		return err
	}
	if err = etaRationalQuotient(m.manager, tmp, qx, &scaled.X, three); err != nil {
		return err
	}
	if err = etaRationalQuotient(m.manager, tmp, qy, &scaled.Y, three); err != nil {
		return err
	}
	if err = m.manager.Copy(&result.X, qx); err != nil {
		return err
	}
	if err = m.manager.Copy(&result.Y, qy); err != nil {
		return err
	}
	if err = out.Copy(m.manager, result); err != nil {
		return err
	}
	if err = m.pairWork.Close(); err != nil {
		return err
	}
	m.pairWork = nil
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}
