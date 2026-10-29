package lp

// Source: src/util/lp/lar_core_solver.h:791-801.

import "github.com/glycerine/g3/pkg/util/numeral"

// FindDeltaForStrictBounds limits the actual model epsilon against each set
// lower and upper bound, in column order. The output publishes only after all
// columns; interrupted exact arithmetic remains owned by this core.
func (c *LARCoreSolver) FindDeltaForStrictBounds(initial, out *numeral.Rational) (err error) {
	if c.stopped || c.strictDelta != nil || c.deltaWork != nil || c.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if initial == nil || out == nil {
		return ErrBounds
	}
	c.strictDelta = NewIndexedRationalVector(c.manager, 1)
	if err = c.copyModelDelta(&c.strictDelta.Data[0], initial); err != nil {
		return err
	}
	for j := uint32(0); j < c.RA.ColumnCount(); j++ {
		low, e := c.LowBoundIsSet(j)
		if e != nil {
			return e
		}
		if low {
			if uint64(j) >= uint64(len(c.Bounds.Low.values)) || uint64(j) >= uint64(len(c.RX.Data)) {
				return ErrBounds
			}
			if err = c.updateDeltaValue(&c.strictDelta.Data[0], &c.Bounds.Low.values[j], &c.RX.Data[j]); err != nil {
				return err
			}
		}
		upper, e := c.UpperBoundIsSet(j)
		if e != nil {
			return e
		}
		if upper {
			if uint64(j) >= uint64(len(c.RX.Data)) || uint64(j) >= uint64(len(c.Bounds.Upper.values)) {
				return ErrBounds
			}
			if err = c.updateDeltaValue(&c.strictDelta.Data[0], &c.RX.Data[j], &c.Bounds.Upper.values[j]); err != nil {
				return err
			}
		}
	}
	if err = c.copyModelDelta(out, &c.strictDelta.Data[0]); err != nil {
		return err
	}
	if err = c.strictDelta.Close(); err != nil {
		return err
	}
	c.strictDelta = nil
	return nil
}

func (c *LARCoreSolver) copyModelDelta(out, value *numeral.Rational) error {
	before, err := c.calls.begin(c.manager)
	if err != nil {
		return err
	}
	err = c.manager.Copy(out, value)
	return c.calls.after(c.manager, before, err)
}
