//go:build g3debug

package lp

// Source: src/test/lp.cpp:2656-2665; lp_settings.hpp:94-123.

import "math"

func (d *LUCheckFileDriver) checkResidual() error {
	b, err := d.LU.BasisDebugMatrix(d.Basis)
	if err != nil {
		return err
	}
	a := make([]float64, d.A.RowCount())
	if a, err = d.A.CopyColumnToVector(26, a); err != nil {
		return err
	}
	cd := NewIndexedFloatVector(0)
	if err = cd.Assign(d.Column); err != nil {
		return err
	}
	if err = b.ApplyLeft(cd.Data); err != nil {
		return err
	}
	for i, da := range cd.Data {
		db := a[i]
		maximum := math.Abs(da)
		if other := math.Abs(db); maximum < other {
			maximum = other
		}
		if maximum > 1 {
			da /= maximum
			db /= maximum
		}
		if math.Abs(da-db) > 0.000001 {
			return ErrAssertion
		}
	}
	return nil
}
