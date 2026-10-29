package lp

// Source: src/util/lp/square_dense_submatrix.hpp:123-173; indexed_value.h:55.

import "github.com/glycerine/g3/pkg/util/numeral"

// Parent helpers own their own numerical invocations. Capture only dense-owned
// assignments here, so a failed parent append/remove/maximum is never adopted
// by both owners. Close this block before its borrowed parent after a failure.
func (m *RationalSquareDenseSubmatrix) beginParentUpdate() error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := m.Parent.ready(); err != nil {
		return err
	}
	m.work = NewIndexedRationalVector(m.manager, 1)
	return nil
}
func (m *RationalSquareDenseSubmatrix) finishParentUpdate(err *error) {
	if *err == nil {
		*err = m.closeWork()
		if *err == nil {
			m.work = nil
		}
	}
}

// Source publishes the row assignment before reaching the column backlink;
// set_value then copies its by-value argument and assigns the column coefficient.
func (m *RationalSquareDenseSubmatrix) publishParentValue(iv *RationalIndexedValue, value *numeral.Rational) (err error) {
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	defer func() { err = m.calls.after(m.manager, b, err) }()
	if err = m.manager.Copy(&iv.Value, value); err != nil {
		return err
	}
	other, err := m.Parent.rowOther(iv)
	if err != nil {
		return err
	}
	arg := &m.work.Data[0]
	if err = m.manager.Copy(arg, &iv.Value); err != nil {
		return err
	}
	if err = m.manager.Copy(&other.Value, arg); err != nil {
		return err
	}
	return m.deleteSlot(0)
}
func (m *RationalSquareDenseSubmatrix) clearPublishedValue(value *numeral.Rational) (err error) {
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	defer func() { err = m.calls.after(m.manager, b, err) }()
	zero := numeral.NewRational()
	return m.manager.Copy(value, &zero)
}

func (m *RationalSquareDenseSubmatrix) updateExistingParentRow(i uint32) error {
	ai, err := m.Parent.AdjustRow(i)
	if err != nil {
		return err
	}
	if uint64(ai) >= uint64(len(m.Parent.Rows)) {
		return ErrBounds
	}
	diagonal := false
	one := numeral.NewRational()
	one.Num = numeral.NewInt(1)
	for k := uint32(0); uint64(k) < uint64(len(m.Parent.Rows[ai])); {
		iv := &m.Parent.Rows[ai][k]
		j, err := m.Parent.AdjustColumnInverse(iv.Index)
		if err != nil {
			return err
		}
		if j < i {
			if err = m.Parent.RemoveElement(ai, k); err != nil {
				return err
			}
			continue
		}
		if j == i {
			if err = m.publishParentValue(iv, &one); err != nil {
				return err
			}
			diagonal = true
		} else {
			v, err := m.cell(i, j)
			if err != nil {
				return err
			}
			zero, err := m.manager.IsZero(v)
			if err != nil {
				return err
			}
			if zero {
				if err = m.Parent.RemoveElement(ai, k); err != nil {
					return err
				}
				continue
			}
			if err = m.publishParentValue(iv, v); err != nil {
				return err
			}
			if err = m.clearPublishedValue(v); err != nil {
				return err
			}
		}
		k++
	}
	if !diagonal {
		aj, err := m.Parent.AdjustColumn(i)
		if err != nil {
			return err
		}
		return m.Parent.AddNewElement(ai, aj, &one)
	}
	return nil
}
func (m *RationalSquareDenseSubmatrix) pushNewParentElements() error {
	for i := m.IndexStart; i < m.Parent.Dimension()-1; i++ {
		ai, err := m.Parent.AdjustRow(i)
		if err != nil {
			return err
		}
		for j := i + 1; j < m.Parent.Dimension(); j++ {
			v, err := m.cell(i, j)
			if err != nil {
				return err
			}
			zero, err := m.manager.IsZero(v)
			if err != nil {
				return err
			}
			if !zero {
				aj, err := m.Parent.AdjustColumn(j)
				if err != nil {
					return err
				}
				if err = m.Parent.AddNewElement(ai, aj, v); err != nil {
					return err
				}
			}
			if err = m.clearPublishedValue(v); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *RationalSquareDenseSubmatrix) UpdateExistingParentRow(i uint32, _ *Settings) (err error) {
	if err = m.beginParentUpdate(); err != nil {
		return err
	}
	defer m.finishParentUpdate(&err)
	return m.updateExistingParentRow(i)
}
func (m *RationalSquareDenseSubmatrix) PushNewParentElements(_ *Settings) (err error) {
	if err = m.beginParentUpdate(); err != nil {
		return err
	}
	defer m.finishParentUpdate(&err)
	return m.pushNewParentElements()
}
func (m *RationalSquareDenseSubmatrix) UpdateParent(_ *Settings) (err error) {
	if err = m.beginParentUpdate(); err != nil {
		return err
	}
	defer m.finishParentUpdate(&err)
	for i := m.IndexStart; i < m.Parent.Dimension(); i++ {
		if err = m.updateExistingParentRow(i); err != nil {
			return err
		}
	}
	if err = m.pushNewParentElements(); err != nil {
		return err
	}
	for i := m.IndexStart; i < m.Parent.Dimension(); i++ {
		row, err := m.Parent.AdjustRow(i)
		if err != nil {
			return err
		}
		if err = m.Parent.SetMaxInRow(row); err != nil {
			return err
		}
	}
	return nil
}
