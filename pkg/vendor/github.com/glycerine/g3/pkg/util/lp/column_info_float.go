package lp

// Source: src/util/lp/column_info.h:27-273, retained floating column metadata.

// FloatColumnInfo is the value state used by the floating LP solver. Unset
// scalar fields have Go's zero value; source reads require their matching flag.
type FloatColumnInfo struct {
	Name                         string
	Low, Upper, FixedValue, Cost float64
	LowSet, UpperSet, Fixed      bool
	LowStrict, UpperStrict       bool
	Index                        uint32
}

const InvalidColumnIndex = ^uint32(0)

func IsValidColumnIndex(i uint32) bool                 { return i < 1<<31 }
func NewFloatColumnInfo(index uint32) *FloatColumnInfo { return &FloatColumnInfo{Index: index} }
func (c *FloatColumnInfo) SetLowBound(v float64)       { c.Low = v; c.LowSet = true }
func (c *FloatColumnInfo) SetUpperBound(v float64)     { c.Upper = v; c.UpperSet = true }
func (c *FloatColumnInfo) SetFixedValue(v float64)     { c.FixedValue = v; c.Fixed = true }
func (c *FloatColumnInfo) ColumnType() ColumnType {
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
func (c *FloatColumnInfo) IsFlipped() bool { return c.UpperSet && !c.LowSet }
func (c *FloatColumnInfo) IsFree() bool    { return !c.LowSet && !c.UpperSet }
func (c *FloatColumnInfo) Shift() float64 {
	if c.Fixed {
		return c.FixedValue
	}
	if c.IsFlipped() {
		return c.Upper
	}
	if c.LowSet {
		return c.Low
	}
	return 0
}
func (c *FloatColumnInfo) GetLowBound() (float64, error) {
	if !c.LowSet {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	return c.Low, nil
}
func (c *FloatColumnInfo) GetUpperBound() (float64, error) {
	if !c.UpperSet {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	return c.Upper, nil
}
func (c *FloatColumnInfo) GetFixedValue() (float64, error) {
	if !c.Fixed {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	return c.FixedValue, nil
}
func (c *FloatColumnInfo) AdjustedUpperBound() (float64, error) {
	hi, err := c.GetUpperBound()
	if err != nil {
		return 0, err
	}
	lo, err := c.GetLowBound()
	if err != nil {
		return 0, err
	}
	return hi - lo, nil
}
func (c *FloatColumnInfo) LowBoundHolds(v float64) bool { return !c.LowSet || v >= c.Low-0.0000001 }
func (c *FloatColumnInfo) UpperBoundHolds(v float64) bool {
	return !c.UpperSet || v <= c.Upper+0.000001
}
func (c *FloatColumnInfo) BoundsHold(v float64) bool {
	return c.LowBoundHolds(v) && c.UpperBoundHolds(v)
}
