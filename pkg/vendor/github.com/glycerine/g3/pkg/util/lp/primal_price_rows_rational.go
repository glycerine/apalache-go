package lp

// Source: src/util/lp/lp_primal_core_solver.h:140-154,174-318.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalPrimalCoreSolver) NeedsToGrow(j uint32) (grow bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled {
		feasible, e := m.ColumnIsFeasible(j)
		if e != nil {
			return false, e
		}
		if feasible {
			return false, ErrAssertion
		}
	}
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed, ColumnLowBound, ColumnBoxed:
		if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.LowBounds)) {
			return false, ErrBounds
		}
		return m.BelowBound(&(*m.X)[j], &(*m.LowBounds)[j])
	default:
		return false, nil
	}
}

func (m *RationalPrimalCoreSolver) MonoidCanDecrease(rc *RationalRowCell) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.monoidCanMove(rc, true)
}
func (m *RationalPrimalCoreSolver) MonoidCanIncrease(rc *RationalRowCell) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.monoidCanMove(rc, false)
}
func (m *RationalPrimalCoreSolver) monoidCanMove(rc *RationalRowCell, decrease bool) (bool, error) {
	if rc == nil {
		return false, ErrBounds
	}
	j := rc.Column
	if sourceDebugEnabled {
		feasible, e := m.ColumnIsFeasible(j)
		if e != nil {
			return false, e
		}
		if !feasible {
			return false, ErrAssertion
		}
	}
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	kind := (*m.ColumnTypes)[j]
	switch kind {
	case ColumnFree:
		return true, nil
	case ColumnFixed:
		return false, nil
	case ColumnLowBound, ColumnUpperBound, ColumnBoxed:
	default:
		return false, nil
	}
	var sign bool
	var err error
	if decrease {
		sign, err = m.manager.IsPos(&rc.Value)
	} else {
		sign, err = m.manager.IsNeg(&rc.Value)
	}
	if err != nil {
		return false, err
	}
	if kind == ColumnLowBound && !sign {
		return true, nil
	}
	if kind == ColumnUpperBound && sign {
		return true, nil
	}
	if uint64(j) >= uint64(len(*m.X)) {
		return false, ErrBounds
	}
	if sign {
		if uint64(j) >= uint64(len(*m.LowBounds)) {
			return false, ErrBounds
		}
		return m.AboveBound(&(*m.X)[j], &(*m.LowBounds)[j])
	}
	if uint64(j) >= uint64(len(*m.UpperBounds)) {
		return false, ErrBounds
	}
	return m.BelowBound(&(*m.X)[j], &(*m.UpperBounds)[j])
}

func (m *RationalPrimalCoreSolver) NumberOfBasicVarsThatMightBecomeInf(j uint32) (count uint32, err error) {
	if err = m.ready(); err != nil {
		return 0, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(m.A.Columns)) {
		return 0, ErrBounds
	}
	for _, cell := range m.A.Columns[j] {
		if uint64(cell.Row) >= uint64(len(*m.Basis)) {
			return count, ErrBounds
		}
		k := (*m.Basis)[cell.Row]
		if uint64(k) >= uint64(len(*m.ColumnTypes)) {
			return count, ErrBounds
		}
		if (*m.ColumnTypes)[k] != ColumnFree {
			count++
		}
	}
	return count, nil
}

func (m *RationalPrimalCoreSolver) copyRowEnteringCoefficient(out *numeral.Rational, rc *RationalRowCell) (err error) {
	if out == nil {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.manager.Copy(out, &rc.Value)
}

func (m *RationalPrimalCoreSolver) FindBeneficialColumnInRowTableauRowsBlandMode(i int32, out *numeral.Rational) (entering int32, err error) {
	if err = m.ready(); err != nil {
		return -1, err
	}
	defer func() { m.stopped = err != nil }()
	entering = -1
	if i < 0 || uint64(i) >= uint64(len(*m.Basis)) {
		return -1, ErrBounds
	}
	bj := (*m.Basis)[i]
	grow, e := m.NeedsToGrow(bj)
	if e != nil {
		return -1, e
	}
	if uint64(i) >= uint64(len(m.A.Rows)) {
		return -1, ErrBounds
	}
	for k := range m.A.Rows[i] {
		rc := &m.A.Rows[i][k]
		if rc.Column == bj {
			continue
		}
		var ok bool
		if grow {
			ok, e = m.MonoidCanDecrease(rc)
		} else {
			ok, e = m.MonoidCanIncrease(rc)
		}
		if e != nil {
			return -1, e
		}
		if !ok {
			continue
		}
		if rc.Column < uint32(entering) {
			entering = int32(rc.Column)
			if err = m.copyRowEnteringCoefficient(out, rc); err != nil {
				return entering, err
			}
		}
	}
	if entering == -1 {
		m.InfRowIndexForTableau = uint32(i)
	}
	return entering, nil
}

func (m *RationalPrimalCoreSolver) FindBeneficialColumnInRowTableauRows(i int32, out *numeral.Rational) (entering int32, err error) {
	if err = m.ready(); err != nil {
		return -1, err
	}
	defer func() { m.stopped = err != nil }()
	if m.BlandModeTableau {
		return m.FindBeneficialColumnInRowTableauRowsBlandMode(i, out)
	}
	var choices []*RationalRowCell
	damageBest, lengthBest := uint32(1000000), uint32(100000000)
	if i < 0 || uint64(i) >= uint64(len(*m.Basis)) {
		return -1, ErrBounds
	}
	bj := (*m.Basis)[i]
	grow, e := m.NeedsToGrow(bj)
	if e != nil {
		return -1, e
	}
	if uint64(i) >= uint64(len(m.A.Rows)) {
		return -1, ErrBounds
	}
	for k := range m.A.Rows[i] {
		rc := &m.A.Rows[i][k]
		j := rc.Column
		if j == bj {
			continue
		}
		var ok bool
		if grow {
			ok, e = m.MonoidCanDecrease(rc)
		} else {
			ok, e = m.MonoidCanIncrease(rc)
		}
		if e != nil {
			return -1, e
		}
		if !ok {
			continue
		}
		damage, e := m.NumberOfBasicVarsThatMightBecomeInf(j)
		if e != nil {
			return -1, e
		}
		if damage < damageBest {
			damageBest = damage
			// The count helper has already reached and checked this column.
			lengthBest = uint32(len(m.A.Columns[j]))
			choices = append(choices[:0], rc)
		} else if damage == damageBest && uint32(len(m.A.Columns[j])) <= lengthBest && m.Settings.RandomNext()%2 != 0 {
			choices = append(choices, rc)
			lengthBest = uint32(len(m.A.Columns[j]))
		}
	}
	if len(choices) == 0 {
		m.InfRowIndexForTableau = uint32(i)
		return -1, nil
	}
	rc := choices[0]
	if len(choices) > 1 {
		rc = choices[m.Settings.RandomNext()%uint32(len(choices))]
	}
	if err = m.copyRowEnteringCoefficient(out, rc); err != nil {
		return -1, err
	}
	return int32(rc.Column), nil
}
