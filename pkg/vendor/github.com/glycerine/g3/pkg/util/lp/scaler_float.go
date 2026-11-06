package lp

// Source: src/util/lp/scaler.h:32-90 and scaler.hpp:38-270.

import (
	"errors"
	"math"
)

// FloatScaler borrows all inputs. Successful scaling changes A, B and the
// column multipliers in place; restoring a rejected matrix is the caller's job.
type FloatScaler struct {
	B                *[]float64
	A                *FloatStaticMatrix
	Minimum, Maximum *float64
	ColumnScale      *[]float64
	Settings         *Settings
}

func NewFloatScaler(b *[]float64, a *FloatStaticMatrix, minimum, maximum *float64, columnScale *[]float64, settings *Settings) (*FloatScaler, error) {
	m := &FloatScaler{b, a, minimum, maximum, columnScale, settings}
	if len(*columnScale) != 0 {
		if err := sourceAssertionFailure(); err != nil {
			return m, err
		}
	}
	n := int(a.ColumnCount())
	if len(*columnScale) > n {
		*columnScale = (*columnScale)[:n]
	}
	for len(*columnScale) < n {
		*columnScale = append(*columnScale, 1)
	}
	return m, nil
}

func (m *FloatScaler) AMin() (float64, error) {
	var result float64
	for i := uint32(0); i < m.A.RowCount(); i++ {
		v, err := m.A.MinAbsRow(i)
		if err != nil {
			return 0, err
		}
		if i == 0 || !(result < v) {
			result = v
		}
	}
	return result, nil
}

func (m *FloatScaler) AMax() (float64, error) {
	var result float64
	for i := uint32(0); i < m.A.RowCount(); i++ {
		v, err := m.A.MaxAbsRow(i)
		if err != nil {
			return 0, err
		}
		if i == 0 || !(v < result) {
			result = v
		}
	}
	return result, nil
}

func (m *FloatScaler) ARatio() (float64, error) {
	min, err := m.AMin()
	if err != nil {
		return 0, err
	}
	max, err := m.AMax()
	if err != nil {
		return 0, err
	}
	if m.Settings.FloatZeroSmall(min) {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	return max / min, nil
}

func (m *FloatScaler) MaxRatioOnRows() (float64, error) {
	result := 1.0
	for i := m.A.RowCount(); i > 0; {
		i--
		den, err := m.A.MinAbsRow(i)
		if err != nil {
			return 0, err
		}
		if m.Settings.FloatZeroSmall(den) {
			if err := sourceAssertionFailure(); err != nil {
				return 0, err
			}
		}
		max, err := m.A.MaxAbsRow(i)
		if err != nil {
			return 0, err
		}
		if v := max / den; v > result {
			result = v
		}
	}
	return result, nil
}

func (m *FloatScaler) MaxRatioOnColumns() (float64, error) {
	result := 1.0
	for i := m.A.ColumnCount(); i > 0; {
		i--
		den, err := m.A.MinAbsColumn(i)
		if err != nil {
			return 0, err
		}
		if m.Settings.FloatZeroSmall(den) {
			continue
		}
		max, err := m.A.MaxAbsColumn(i)
		if err != nil {
			return 0, err
		}
		if v := max / den; v > result {
			result = v
		}
	}
	return result, nil
}

func (m *FloatScaler) ScaleRowsWithGeometricMean() error {
	for i := m.A.RowCount(); i > 0; {
		i--
		max, err := m.A.MaxAbsRow(i)
		if err != nil {
			return err
		}
		min, err := m.A.MinAbsRow(i)
		if err != nil {
			return err
		}
		if !(max > 0 && min > 0) {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
		}
		if max == 0 || min == 0 {
			continue
		}
		gm := math.Sqrt(max * min)
		if FloatEpsilonSmall(gm, 0.01) {
			continue
		}
		if err := m.A.MultiplyRow(i, 1/gm); err != nil {
			return err
		}
		if uint64(i) >= uint64(len(*m.B)) {
			return ErrBounds
		}
		(*m.B)[i] /= gm
	}
	return nil
}

func (m *FloatScaler) ScaleColumnsWithGeometricMean() error {
	for i := m.A.ColumnCount(); i > 0; {
		i--
		max, err := m.A.MaxAbsColumn(i)
		if err != nil {
			return err
		}
		min, err := m.A.MinAbsColumn(i)
		if err != nil {
			return err
		}
		den := math.Sqrt(max * min)
		if FloatEpsilonSmall(den, 0.01) {
			continue
		}
		gm := 1 / den
		if uint64(i) >= uint64(len(*m.ColumnScale)) {
			return ErrBounds
		}
		cs := (*m.ColumnScale)[i] * gm
		if FloatEpsilonSmall(cs, 0.1) {
			continue
		}
		if err := m.A.MultiplyColumn(i, gm); err != nil {
			return err
		}
		(*m.ColumnScale)[i] = cs
	}
	return nil
}

func (m *FloatScaler) ScaleOnceForRatio() error {
	r, err := m.MaxRatioOnRows()
	if err != nil {
		return err
	}
	c, err := m.MaxRatioOnColumns()
	if err != nil {
		return err
	}
	if r > c {
		if err := m.ScaleRowsWithGeometricMean(); err != nil {
			return err
		}
		return m.ScaleColumnsWithGeometricMean()
	}
	if err := m.ScaleColumnsWithGeometricMean(); err != nil {
		return err
	}
	return m.ScaleRowsWithGeometricMean()
}

func (m *FloatScaler) ScaleWithRatio() (bool, error) {
	ratio, err := m.ARatio()
	if err != nil {
		return false, err
	}
	reps := m.Settings.RepsInScaler
	for {
		if err := m.ScaleOnceForRatio(); err != nil {
			return false, err
		}
		next, err := m.ARatio()
		if err != nil {
			return false, err
		}
		// Source compares every iteration with the ORIGINAL ratio.
		if next >= 0.9*ratio {
			break
		}
		old := reps
		reps--
		if old == 0 {
			break
		}
	}
	if err := m.BringRowsAndColumnsMaximumsToOne(); err != nil {
		return false, err
	}
	return true, nil
}

func (m *FloatScaler) BringRowMaximumsToOne() error {
	for i := m.A.RowCount(); i > 0; {
		i--
		v, err := m.A.MaxAbsRow(i)
		if err != nil {
			return err
		}
		if m.Settings.FloatZeroSmall(v) {
			continue
		}
		if err := m.A.MultiplyRow(i, 1/v); err != nil {
			return err
		}
		if uint64(i) >= uint64(len(*m.B)) {
			return ErrBounds
		}
		(*m.B)[i] /= v
	}
	return nil
}

func (m *FloatScaler) BringColumnMaximumsToOne() error {
	for i := m.A.ColumnCount(); i > 0; {
		i--
		max, err := m.A.MaxAbsColumn(i)
		if err != nil {
			return err
		}
		if m.Settings.FloatZeroSmall(max) {
			continue
		}
		v := 1 / max
		if err := m.A.MultiplyColumn(i, v); err != nil {
			return err
		}
		if uint64(i) >= uint64(len(*m.ColumnScale)) {
			return ErrBounds
		}
		(*m.ColumnScale)[i] *= v
	}
	return nil
}

func (m *FloatScaler) BringRowsAndColumnsMaximumsToOne() error {
	r, err := m.MaxRatioOnRows()
	if err != nil {
		return err
	}
	c, err := m.MaxRatioOnColumns()
	if err != nil {
		return err
	}
	if r > c {
		if err := m.BringRowMaximumsToOne(); err != nil {
			return err
		}
		return m.BringColumnMaximumsToOne()
	}
	if err := m.BringColumnMaximumsToOne(); err != nil {
		return err
	}
	return m.BringRowMaximumsToOne()
}

func (m *FloatScaler) ScaleWithLogBalance() (bool, error) {
	balance, err := m.A.Balance()
	if err != nil {
		return false, err
	}
	before := balance
	for i := 0; i < 10; i++ {
		if err := m.ScaleRows(); err != nil {
			return false, err
		}
		if err := m.ScaleColumns(); err != nil {
			return false, err
		}
		next, err := m.A.Balance()
		if err != nil {
			return false, err
		}
		if next < 0.9*balance {
			balance = next
		} else {
			balance = next
			break
		}
	}
	return balance <= before, nil
}

func (m *FloatScaler) Scale() (bool, error) {
	if m.Settings.ScaleWithRatio {
		return m.ScaleWithRatio()
	}
	return m.ScaleWithLogBalance()
}

// This guards only a source nonterminating domain (for example +Inf divided
// by two, or zero compared with a negative maximum), after its first loop body.
var ErrLPScalingStalled = errors.New("lp: source power-of-two scaling cannot progress")

func scalerPowerOfTwo(value, min, max float64) (float64, error) {
	alpha := 1.0
	if value < min {
		for {
			alpha *= 2
			value *= 2
			if !(value < min) {
				break
			}
		}
	} else if value > max {
		for {
			old := value
			alpha /= 2
			value /= 2
			if !(value > max) {
				break
			}
			if value == old {
				return 0, ErrLPScalingStalled
			}
		}
	}
	return alpha, nil
}

func (m *FloatScaler) ScaleRow(i uint32) error {
	max, err := m.A.MaxAbsRow(i)
	if err != nil {
		return err
	}
	if uint64(i) >= uint64(len(*m.B)) {
		return ErrBounds
	}
	b := math.Abs((*m.B)[i])
	if max < b {
		max = b
	}
	if max == 0 {
		return nil
	}
	if !(max < *m.Minimum || max > *m.Maximum) {
		return nil
	}
	alpha, err := scalerPowerOfTwo(max, *m.Minimum, *m.Maximum)
	if err != nil {
		return err
	}
	if err := m.A.MultiplyRow(i, alpha); err != nil {
		return err
	}
	(*m.B)[i] *= alpha
	return nil
}

func (m *FloatScaler) ScaleColumn(i uint32) error {
	max, err := m.A.MaxAbsColumn(i)
	if err != nil {
		return err
	}
	if max == 0 {
		return nil
	}
	if !(max < *m.Minimum || max > *m.Maximum) {
		return nil
	}
	alpha, err := scalerPowerOfTwo(max, *m.Minimum, *m.Maximum)
	if err != nil {
		return err
	}
	if err := m.A.MultiplyColumn(i, alpha); err != nil {
		return err
	}
	if uint64(i) >= uint64(len(*m.ColumnScale)) {
		return ErrBounds
	}
	// Source assigns, rather than multiplying an earlier column scale.
	(*m.ColumnScale)[i] = alpha
	return nil
}

func (m *FloatScaler) ScaleRows() error {
	for i := uint32(0); i < m.A.RowCount(); i++ {
		if err := m.ScaleRow(i); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatScaler) ScaleColumns() error {
	for i := uint32(0); i < m.A.ColumnCount(); i++ {
		if err := m.ScaleColumn(i); err != nil {
			return err
		}
	}
	return nil
}
