package lp

// Source: src/util/lp/column_info.h, retained column_info<mpq> LP consumers.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalColumnInfo owns exact bounds and cost in a borrowed manager. Unset
// fields are initialized zero numerals. Do not copy the owner or its numerals;
// getter outputs must outlive Close after an interrupted copy.
type RationalColumnInfo struct {
	Name                         string
	Low, Upper, FixedValue, Cost numeral.Rational
	LowSet, UpperSet, Fixed      bool
	LowStrict, UpperStrict       bool
	Index                        uint32
	manager                      *numeral.RationalManager
	calls                        exactEtaCalls
	work, boundWork              *IndexedRationalVector
	stopped, closed              bool
}

func NewRationalColumnInfo(m *numeral.RationalManager, index uint32) *RationalColumnInfo {
	return &RationalColumnInfo{manager: m, Index: index, Low: numeral.NewRational(), Upper: numeral.NewRational(), FixedValue: numeral.NewRational(), Cost: numeral.NewRational()}
}
func (c *RationalColumnInfo) ready() error {
	if c.closed || c.stopped || c.work != nil || c.boundWork != nil || c.calls.pending() {
		return ErrExactVectorPending
	}
	if c.manager == nil {
		return ErrExactVectorOwner
	}
	return nil
}
func (c *RationalColumnInfo) copyValue(out, value *numeral.Rational) error {
	if out == nil || value == nil {
		return ErrBounds
	}
	b, err := c.calls.begin(c.manager)
	if err != nil {
		return err
	}
	err = c.manager.Copy(out, value)
	return c.calls.after(c.manager, b, err)
}
func (c *RationalColumnInfo) SetLowBound(v *numeral.Rational) (err error) {
	if err = c.ready(); err != nil {
		return err
	}
	defer func() { c.stopped = err != nil }()
	if err = c.copyValue(&c.Low, v); err != nil {
		return err
	}
	c.LowSet = true
	return nil
}
func (c *RationalColumnInfo) SetUpperBound(v *numeral.Rational) (err error) {
	if err = c.ready(); err != nil {
		return err
	}
	defer func() { c.stopped = err != nil }()
	if err = c.copyValue(&c.Upper, v); err != nil {
		return err
	}
	c.UpperSet = true
	return nil
}
func (c *RationalColumnInfo) SetCost(v *numeral.Rational) (err error) {
	if err = c.ready(); err != nil {
		return err
	}
	defer func() { c.stopped = err != nil }()
	return c.copyValue(&c.Cost, v)
}
func (c *RationalColumnInfo) SetFixedValue(v *numeral.Rational) (err error) {
	if err = c.ready(); err != nil {
		return err
	}
	defer func() { c.stopped = err != nil }()
	if v == nil {
		return ErrBounds
	}
	// The by-value source parameter is constructed before entering the body.
	c.work = NewIndexedRationalVector(c.manager, 1)
	if err = c.work.copySlot(&c.work.Data[0], v); err != nil {
		return err
	}
	c.Fixed = true
	if err = c.copyValue(&c.FixedValue, &c.work.Data[0]); err != nil {
		return err
	}
	if err = c.work.Close(); err != nil {
		return err
	}
	c.work = nil
	return nil
}
func (c *RationalColumnInfo) ColumnType() ColumnType {
	if c.Fixed {
		return ColumnFixed
	}
	if c.LowSet {
		if c.UpperSet {
			return ColumnBoxed
		}
		return ColumnLowBound
	}
	if c.UpperSet {
		return ColumnUpperBound
	}
	return ColumnFree
}
func (c *RationalColumnInfo) IsFlipped() bool { return c.UpperSet && !c.LowSet }
func (c *RationalColumnInfo) IsFree() bool    { return !c.LowSet && !c.UpperSet }
func (c *RationalColumnInfo) readValue(out, value *numeral.Rational, set bool) (err error) {
	if err = c.ready(); err != nil {
		return err
	}
	defer func() { c.stopped = err != nil }()
	if !set {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	return c.copyValue(out, value)
}
func (c *RationalColumnInfo) GetLowBound(out *numeral.Rational) error {
	return c.readValue(out, &c.Low, c.LowSet)
}
func (c *RationalColumnInfo) GetUpperBound(out *numeral.Rational) error {
	return c.readValue(out, &c.Upper, c.UpperSet)
}
func (c *RationalColumnInfo) GetFixedValue(out *numeral.Rational) error {
	return c.readValue(out, &c.FixedValue, c.Fixed)
}
func (c *RationalColumnInfo) GetCost(out *numeral.Rational) error {
	return c.readValue(out, &c.Cost, true)
}
func (c *RationalColumnInfo) Shift(out *numeral.Rational) error {
	value := &c.Low
	if c.Fixed {
		value = &c.FixedValue
	} else if c.IsFlipped() {
		value = &c.Upper
	} else if !c.LowSet {
		zero := numeral.NewRational()
		return c.readValue(out, &zero, true)
	}
	return c.readValue(out, value, true)
}
func (c *RationalColumnInfo) AdjustedUpperBound(out *numeral.Rational) (err error) {
	if err = c.ready(); err != nil {
		return err
	}
	defer func() { c.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	// Keep both getter results and the copied-left subtraction temporary alive
	// until the returned reference has been copied into the expression result.
	c.work = NewIndexedRationalVector(c.manager, 4)
	for i, field := range []struct {
		set   bool
		value *numeral.Rational
	}{{c.UpperSet, &c.Upper}, {c.LowSet, &c.Low}} {
		if !field.set {
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
		}
		if err = c.work.copySlot(&c.work.Data[i], field.value); err != nil {
			return err
		}
	}
	b, e := c.calls.begin(c.manager)
	if e != nil {
		return e
	}
	e = etaRationalDifference(c.manager, &c.work.Data[3], &c.work.Data[2], &c.work.Data[0], &c.work.Data[1])
	if err = c.calls.after(c.manager, b, e); err != nil {
		return err
	}
	if err = c.copyValue(out, &c.work.Data[2]); err != nil {
		return err
	}
	if err = c.work.Close(); err != nil {
		return err
	}
	c.work = nil
	return nil
}

// The source uses T(double) for tolerances. rational(double) is UNREACHABLE:
// debug asserts, native leaves the default rational zero. Preserve this exact
// specialization rather than introducing floating tolerance into exact LPs.
func (c *RationalColumnInfo) boundHolds(v *numeral.Rational, upper bool) (bool, error) {
	if v == nil {
		return false, ErrBounds
	}
	c.boundWork = NewIndexedRationalVector(c.manager, 3)
	if err := c.boundWork.copySlot(&c.boundWork.Data[0], v); err != nil {
		return false, err
	}
	set := c.LowSet
	if upper {
		set = c.UpperSet
	}
	if !set {
		if err := c.boundWork.Close(); err != nil {
			return false, err
		}
		c.boundWork = nil
		return true, nil
	}
	if err := sourceAssertionFailure(); err != nil {
		return false, err
	}
	zero := numeral.NewRational()
	b, err := c.calls.begin(c.manager)
	if err != nil {
		return false, err
	}
	if upper {
		err = etaRationalSum(c.manager, &c.boundWork.Data[2], &c.boundWork.Data[1], &c.Upper, &zero)
	} else {
		err = etaRationalDifference(c.manager, &c.boundWork.Data[2], &c.boundWork.Data[1], &c.Low, &zero)
	}
	if err = c.calls.after(c.manager, b, err); err != nil {
		return false, err
	}
	b, err = c.calls.begin(c.manager)
	if err != nil {
		return false, err
	}
	cmp, e := c.manager.Compare(&c.boundWork.Data[0], &c.boundWork.Data[1])
	if err = c.calls.after(c.manager, b, e); err != nil {
		return false, err
	}
	holds := cmp >= 0
	if upper {
		holds = cmp <= 0
	}
	if err = c.boundWork.Close(); err != nil {
		return false, err
	}
	c.boundWork = nil
	return holds, nil
}
func (c *RationalColumnInfo) LowBoundHolds(v *numeral.Rational) (holds bool, err error) {
	if err = c.ready(); err != nil {
		return false, err
	}
	defer func() { c.stopped = err != nil }()
	return c.boundHolds(v, false)
}
func (c *RationalColumnInfo) UpperBoundHolds(v *numeral.Rational) (holds bool, err error) {
	if err = c.ready(); err != nil {
		return false, err
	}
	defer func() { c.stopped = err != nil }()
	return c.boundHolds(v, true)
}
func (c *RationalColumnInfo) BoundsHold(v *numeral.Rational) (holds bool, err error) {
	if err = c.ready(); err != nil {
		return false, err
	}
	defer func() { c.stopped = err != nil }()
	if v == nil {
		return false, ErrBounds
	}
	c.work = NewIndexedRationalVector(c.manager, 1)
	if err = c.work.copySlot(&c.work.Data[0], v); err != nil {
		return false, err
	}
	holds, err = c.boundHolds(&c.work.Data[0], false)
	if err != nil {
		return false, err
	}
	if holds {
		holds, err = c.boundHolds(&c.work.Data[0], true)
		if err != nil {
			return false, err
		}
	}
	if err = c.work.Close(); err != nil {
		return false, err
	}
	c.work = nil
	return holds, nil
}
func (c *RationalColumnInfo) Close() error {
	if c.closed {
		return nil
	}
	if c.manager != nil {
		if err := c.calls.cleanupRaw(c.manager); err != nil {
			return err
		}
		if c.boundWork != nil {
			if err := c.boundWork.Close(); err != nil {
				return err
			}
			c.boundWork = nil
		}
		if c.work != nil {
			if err := c.work.Close(); err != nil {
				return err
			}
			c.work = nil
		}
		for _, v := range []*numeral.Rational{&c.Cost, &c.FixedValue, &c.Upper, &c.Low} {
			if err := c.manager.Del(v); err != nil {
				return err
			}
			*v = numeral.NewRational()
		}
		if err := c.calls.cleanupRoots(c.manager); err != nil {
			return err
		}
	}
	c.closed = true
	return nil
}
