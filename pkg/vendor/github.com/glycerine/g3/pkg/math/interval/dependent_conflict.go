package interval

// Source: src/math/interval/dep_intervals.h:294-332.

func (v *DependentRationalInterval) ZeroConflict(initial *IntervalDependency) (explanation []uint32, separated bool, err error) {
	lower, err := v.SeparatedFromZeroLower()
	if err != nil {
		return nil, false, err
	}
	if lower {
		return v.manager.Dependencies.Linearize(v.manager.Join(initial, v.LowerDep)), true, nil
	}
	upper, err := v.SeparatedFromZeroUpper()
	if err != nil {
		return nil, false, err
	}
	if upper {
		return v.manager.Dependencies.Linearize(v.manager.Join(initial, v.UpperDep)), true, nil
	}
	return nil, false, nil
}

func (v *DependentRationalInterval) JoinZeroMultiplicationDeps() error {
	if err := v.open(); err != nil {
		return err
	}
	v.LowerDep = v.manager.Join(v.LowerDep, v.UpperDep)
	v.UpperDep = v.LowerDep
	return nil
}
