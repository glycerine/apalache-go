package interval

// Source: src/math/interval/dep_intervals.h:41-52, 180-200, 429-435.

import "github.com/glycerine/g3/pkg/arena"

type DependentRationalInterval struct {
	manager            *DependentIntervalManager
	Value              *RationalInterval
	LowerDep, UpperDep *IntervalDependency
	closed             bool
}

func (v *DependentRationalInterval) open() error {
	if v == nil || v.manager == nil || v.Value == nil || v.manager.Numbers != v.Value.numbers {
		return arena.ErrInvalidSlot
	}
	if v.closed {
		return arena.ErrClosed
	}
	return v.Value.open()
}

func (v *DependentRationalInterval) Close() error {
	if v != nil && v.closed {
		return nil
	}
	if err := v.open(); err != nil {
		return err
	}
	err := v.Value.Close()
	if err == nil {
		v.closed, v.LowerDep, v.UpperDep = true, nil, nil
	}
	return err
}

func (v *DependentRationalInterval) LowerDependencies() []uint32 {
	if v == nil || v.manager == nil {
		return nil
	}
	return v.manager.Dependencies.Linearize(v.LowerDep)
}

func (v *DependentRationalInterval) UpperDependencies() []uint32 {
	if v == nil || v.manager == nil {
		return nil
	}
	return v.manager.Dependencies.Linearize(v.UpperDep)
}
