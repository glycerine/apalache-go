package lp

// Source: src/util/lp/lp_core_solver_base.hpp:920-935;
// lp_settings.h:231-233,273-275. Precise rational specialization.

import "github.com/glycerine/g3/pkg/util/numeral"

// PivotsInColumnAndRowAreDifferent returns 0 for equality, 1 for zero/drift,
// and 2 for opposite signs. Precise Harris tolerance tests exact zero, but
// still evaluates the source normalized difference expression in full.
func (m *RationalCoreSolverBase) PivotsInColumnAndRowAreDifferent(entering, leaving int32) (result int, err error) {
	if err = m.ready(); err != nil {
		return 0, err
	}
	defer func() { m.stopped = err != nil }()
	en, lv := uint32(entering), uint32(leaving)
	if uint64(lv) >= uint64(len(*m.Heading)) {
		return 0, ErrBounds
	}
	row := uint32((*m.Heading)[lv])
	if uint64(row) >= uint64(len(m.Ed.Data)) || uint64(en) >= uint64(len(m.PivotRow.Data)) {
		return 0, ErrBounds
	}
	column, pivot := &m.Ed.Data[row], &m.PivotRow.Data[en]
	zero, e := m.manager.IsZero(column)
	if e != nil {
		return 0, e
	}
	if zero {
		return 1, nil
	}
	zero, e = m.manager.IsZero(pivot)
	if e != nil {
		return 0, e
	}
	if zero {
		return 1, nil
	}
	cs, e := m.manager.Sign(column)
	if e != nil {
		return 0, e
	}
	ps, e := m.manager.Sign(pivot)
	if e != nil {
		return 0, e
	}
	if (cs < 0 && ps > 0) || (cs > 0 && ps < 0) {
		return 2, nil
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return 0, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.pivotWork = NewIndexedRationalVector(m.manager, 9)
	tmp, absRow, one, denom, diff, quotient, normalized, ten, scaled := &m.pivotWork.Data[0], &m.pivotWork.Data[1], &m.pivotWork.Data[2], &m.pivotWork.Data[3], &m.pivotWork.Data[4], &m.pivotWork.Data[5], &m.pivotWork.Data[6], &m.pivotWork.Data[7], &m.pivotWork.Data[8]
	if err = etaRationalDifference(m.manager, tmp, diff, column, pivot); err != nil {
		return 0, err
	}
	if err = m.manager.Copy(absRow, pivot); err != nil {
		return 0, err
	}
	if err = m.manager.Abs(absRow); err != nil {
		return 0, err
	}
	if err = m.manager.SetInt32(one, 1); err != nil {
		return 0, err
	}
	if err = etaRationalSum(m.manager, tmp, denom, one, absRow); err != nil {
		return 0, err
	}
	if err = etaRationalQuotient(m.manager, tmp, quotient, diff, denom); err != nil {
		return 0, err
	}
	if err = m.manager.Copy(normalized, quotient); err != nil {
		return 0, err
	}
	if err = m.manager.Abs(normalized); err != nil {
		return 0, err
	}
	// Complete the initialization full expression before constructing T(10).
	for _, i := range []int{5, 3, 2, 1, 4} {
		if err = m.manager.Del(&m.pivotWork.Data[i]); err != nil {
			return 0, err
		}
		m.pivotWork.Data[i] = numeral.NewRational()
	}
	if err = m.manager.SetInt32(ten, 10); err != nil {
		return 0, err
	}
	if err = etaRationalQuotient(m.manager, tmp, scaled, normalized, ten); err != nil {
		return 0, err
	}
	zero, e = m.manager.IsZero(scaled)
	if e != nil {
		return 0, e
	}
	if err = m.pivotWork.Close(); err != nil {
		return 0, err
	}
	m.pivotWork = nil
	if !zero {
		return 1, nil
	}
	return 0, nil
}
