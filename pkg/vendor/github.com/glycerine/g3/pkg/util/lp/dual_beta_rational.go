package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:207-210,357-375,411-431.
// Retained scalar mpq/mpq beta recurrence and actual LU reinitialization.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalDualCoreSolver) DSEFTran() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.Factorization == nil || m.PivotRowB1 == nil {
		return ErrBounds
	}
	return m.Factorization.SolveByForTIndexedOnly(m.PivotRowB1, m.Settings)
}
func (m *RationalDualCoreSolver) InitBetaPrecisely(i uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.betaVector != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	m.betaVector = NewIndexedRationalVector(m.manager, m.A.RowCount())
	if uint64(i) >= uint64(len(m.betaVector.Data)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.SetInt32(&m.betaVector.Data[i], 1)
	if err = m.calls.after(m.manager, before, err); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	if err = m.Factorization.SolveYBWithErrorCheck(m.betaVector.Data, *m.Basis); err != nil {
		return err
	}
	before, e = m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 3)
	beta, tmp, product := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2]
	for k := range m.betaVector.Data {
		v := &m.betaVector.Data[k]
		if err = etaRationalProduct(m.manager, tmp, product, v, v); err != nil {
			return err
		}
		if err = m.manager.Add(beta, product, beta); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	if m.Betas == nil || uint64(i) >= uint64(len(m.Betas.Data)) {
		return ErrBounds
	}
	if err = m.manager.Copy(&m.Betas.Data[i], beta); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	if err = m.betaVector.Close(); err != nil {
		return err
	}
	m.betaVector = nil
	return nil
}
func (m *RationalDualCoreSolver) InitBetasPrecisely() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	for i := m.A.RowCount(); i > 0; {
		i--
		if err = m.InitBetaPrecisely(i); err != nil {
			return err
		}
	}
	return nil
}

// rational(double) is source UNREACHABLE. As with exact ColumnInfo tolerance
// construction, checked builds assert; native retains the default zero value.
// No floating approximation or invented positive exact floor is introduced.
func (m *RationalDualCoreSolver) UpdateBetas() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 14)
	w := m.priceWork.Data
	one, tmp, inv, floor := &w[0], &w[1], &w[2], &w[3]
	first, expression, betaR, k, minusTwo := &w[4], &w[5], &w[6], &w[7], &w[8]
	a, aBeta, kRow, inner, outer := &w[9], &w[10], &w[11], &w[12], &w[13]
	if err = m.manager.SetInt32(one, 1); err != nil {
		return err
	}
	q := uint32(m.Q)
	if m.PivotRow == nil || uint64(q) >= uint64(len(m.PivotRow.Data)) {
		return ErrBounds
	}
	if err = etaRationalQuotient(m.manager, tmp, inv, one, &m.PivotRow.Data[q]); err != nil {
		return err
	}
	if err = m.manager.Del(one); err != nil {
		return err
	}
	*one = numeral.NewRational()
	if err = sourceAssertionFailure(); err != nil {
		return err
	}
	if m.Betas == nil || m.R < 0 || uint64(m.R) >= uint64(len(m.Betas.Data)) {
		return ErrBounds
	}
	if err = etaRationalProduct(m.manager, tmp, first, &m.Betas.Data[m.R], inv); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, expression, first, inv); err != nil {
		return err
	}
	cmp, e := m.manager.Compare(floor, expression)
	if e != nil {
		return e
	}
	selected := floor
	if cmp < 0 {
		selected = expression
	}
	if err = m.manager.Copy(&m.Betas.Data[m.R], selected); err != nil {
		return err
	}
	if err = m.manager.Copy(betaR, &m.Betas.Data[m.R]); err != nil {
		return err
	}
	for _, v := range []*numeral.Rational{first, expression} {
		if err = m.manager.Del(v); err != nil {
			return err
		}
		*v = numeral.NewRational()
	}
	if err = m.manager.SetInt32(minusTwo, -2); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, k, minusTwo, inv); err != nil {
		return err
	}
	if err = m.manager.Del(minusTwo); err != nil {
		return err
	}
	*minusTwo = numeral.NewRational()
	for i := m.A.RowCount(); i > 0; {
		i--
		if int32(i) == m.R {
			continue
		}
		if m.Ed == nil || uint64(i) >= uint64(len(m.Ed.Data)) {
			return ErrBounds
		}
		if err = m.manager.Copy(a, &m.Ed.Data[i]); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, tmp, aBeta, a, betaR); err != nil {
			return err
		}
		if m.PivotRowB1 == nil || uint64(i) >= uint64(len(m.PivotRowB1.Data)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, kRow, k, &m.PivotRowB1.Data[i]); err != nil {
			return err
		}
		if err = etaRationalSum(m.manager, tmp, inner, aBeta, kRow); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, tmp, outer, a, inner); err != nil {
			return err
		}
		if uint64(i) >= uint64(len(m.Betas.Data)) {
			return ErrBounds
		}
		if err = m.manager.Add(&m.Betas.Data[i], outer, &m.Betas.Data[i]); err != nil {
			return err
		}
		for _, v := range []*numeral.Rational{aBeta, kRow, inner, outer} {
			if err = m.manager.Del(v); err != nil {
				return err
			}
			*v = numeral.NewRational()
		}
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		cmp, e = m.manager.Compare(&m.Betas.Data[i], floor)
		if e != nil {
			return e
		}
		if cmp < 0 {
			if err = m.manager.Copy(&m.Betas.Data[i], floor); err != nil {
				return err
			}
		}
		if err = m.manager.Del(a); err != nil {
			return err
		}
		*a = numeral.NewRational()
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}
