package interval

// Source: src/math/interval/dep_intervals.h:184-215, 267-290 and
// src/math/interval/dep_intervals.cpp:29-67.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (v *DependentRationalInterval) SetScalar(value *numeral.Rational) error {
	if err := v.open(); err != nil {
		return err
	}
	if err := v.manager.Numbers.Check(value); err != nil {
		return err
	}
	return v.Value.SetFinite(value, value, false, false)
}

func (v *DependentRationalInterval) SetZero(dependency *IntervalDependency) error {
	zero := numeral.NewRational()
	if err := v.SetScalar(&zero); err != nil {
		return err
	}
	v.LowerDep, v.UpperDep = dependency, dependency
	return nil
}

func (v *DependentRationalInterval) CopyFrom(source *DependentRationalInterval, withDeps bool) error {
	if err := v.checkPeer(source); err != nil {
		return err
	}
	if err := v.Value.SetFrom(source.Value); err != nil {
		return err
	}
	if withDeps {
		v.LowerDep, v.UpperDep = source.LowerDep, source.UpperDep
	}
	return nil
}

func (v *DependentRationalInterval) AddScalar(value *numeral.Rational) error {
	if err := v.open(); err != nil {
		return err
	}
	if err := v.manager.Numbers.Check(value); err != nil {
		return err
	}
	if !v.Value.lowerInf {
		if err := v.manager.Numbers.Add(&v.Value.lower, value, &v.Value.lower); err != nil {
			return err
		}
	}
	if !v.Value.upperInf {
		return v.manager.Numbers.Add(&v.Value.upper, value, &v.Value.upper)
	}
	return nil
}

func (v *DependentRationalInterval) checkPeer(other *DependentRationalInterval) error {
	if err := v.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if v.manager != other.manager {
		return arena.ErrInvalidSlot
	}
	return nil
}
