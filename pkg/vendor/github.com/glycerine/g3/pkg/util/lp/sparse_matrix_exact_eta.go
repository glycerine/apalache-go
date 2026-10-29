package lp

// Source: src/util/lp/sparse_matrix.hpp:1119-1167, retained mpq coefficient types.

import "github.com/glycerine/g3/pkg/util/numeral"

// FillEtaMatrix returns an independently owned eta for logical column j. Unit
// columns return nil,true and exact zero diagonals return nil,false. On error,
// close any returned eta before closing the matrix: eta owns its child copies
// and division, while the matrix retains its unary-negation temporary. Source
// coefficients are borrowed and remain unchanged. Do not reuse a stopped matrix.
func (m *RationalSparseMatrix) FillEtaMatrix(j uint32) (eta *RationalEtaMatrix, ok bool, err error) {
	if err = m.ready(); err != nil {
		return nil, false, err
	}
	column, err := m.AdjustColumn(j)
	if err != nil {
		return nil, false, err
	}
	if uint64(column) >= uint64(len(m.Columns)) {
		return nil, false, ErrBounds
	}
	values := m.Columns[column].Values
	m.work = NewIndexedRationalVector(m.manager, 1)
	// Capture only our own expressions; eta owns receipts acquired by its calls.
	defer func() {
		if err == nil {
			err = m.work.Close()
			if err == nil {
				m.work = nil
			}
		}
		if err != nil {
			ok = false
		}
	}()
	unit := true
	one := numeral.NewRational()
	one.Num = numeral.NewInt(1)
	for k := range values {
		v := &values[k]
		i, e := m.AdjustRowInverse(v.Index)
		if e != nil {
			return nil, false, e
		}
		if i > j {
			unit = false
			break
		}
		if i == j {
			equal, e := m.manager.Equal(&v.Value, &one)
			if e != nil {
				return nil, false, e
			}
			if !equal {
				unit = false
				break
			}
		}
	}
	if unit {
		return nil, true, nil
	}
	eta = NewRationalEtaMatrix(m.manager, j, m.Dimension())
	for k := range values {
		v := &values[k]
		i, e := m.AdjustRowInverse(v.Index)
		if e != nil {
			return eta, false, e
		}
		if i < j {
			continue
		}
		if i > j {
			b, e := m.calls.begin(m.manager)
			if e != nil {
				return eta, false, e
			}
			negative := &m.work.Data[0]
			e = m.manager.Copy(negative, &v.Value)
			if e == nil {
				e = m.manager.Neg(negative)
			}
			if err = m.calls.after(m.manager, b, e); err != nil {
				return eta, false, err
			}
			if err = eta.Append(i, negative); err != nil {
				return eta, false, err
			}
			if err = m.manager.Del(negative); err != nil {
				return eta, false, err
			}
			*negative = numeral.NewRational()
		} else {
			accepted, e := eta.SetDiagonal(&v.Value)
			if e != nil {
				return eta, false, e
			}
			if !accepted {
				if err = eta.Close(); err != nil {
					return eta, false, err
				}
				return nil, false, nil
			}
		}
	}
	err = eta.DivideByDiagonal()
	return eta, err == nil, err
}
