package interval

// Source: src/math/interval/dep_intervals.h:333-391.

func (v *DependentRationalInterval) Intersect(a, b *DependentRationalInterval, withDeps bool) error {
	if err := v.checkOperands(a, b); err != nil {
		return err
	}
	lowerSource, err := chooseIntersectionLower(a, b)
	if err != nil {
		return err
	}
	upperSource, err := chooseIntersectionUpper(a, b)
	if err != nil {
		return err
	}
	if lowerSource != nil {
		if err = v.manager.Numbers.Copy(&v.Value.lower, &lowerSource.Value.lower); err != nil {
			return err
		}
		v.Value.lowerInf, v.Value.lowerOpen = false, lowerSource.Value.lowerOpen
		if withDeps {
			v.LowerDep = lowerSource.LowerDep
		}
	}
	if upperSource != nil {
		if err = v.manager.Numbers.Copy(&v.Value.upper, &upperSource.Value.upper); err != nil {
			return err
		}
		v.Value.upperInf, v.Value.upperOpen = false, upperSource.Value.upperOpen
		if withDeps {
			v.UpperDep = upperSource.UpperDep
		}
	}
	return nil
}

func chooseIntersectionLower(a, b *DependentRationalInterval) (*DependentRationalInterval, error) {
	if a.Value.lowerInf {
		if b.Value.lowerInf {
			return nil, nil
		}
		return b, nil
	}
	if b.Value.lowerInf {
		return a, nil
	}
	cmp, err := a.manager.Numbers.Compare(&a.Value.lower, &b.Value.lower)
	if err != nil {
		return nil, err
	}
	if cmp < 0 {
		return b, nil
	}
	if cmp > 0 || a.Value.lowerOpen {
		return a, nil
	}
	return b, nil
}

func chooseIntersectionUpper(a, b *DependentRationalInterval) (*DependentRationalInterval, error) {
	if a.Value.upperInf {
		if b.Value.upperInf {
			return nil, nil
		}
		return b, nil
	}
	if b.Value.upperInf {
		return a, nil
	}
	cmp, err := a.manager.Numbers.Compare(&a.Value.upper, &b.Value.upper)
	if err != nil {
		return nil, err
	}
	if cmp > 0 {
		return b, nil
	}
	if cmp < 0 || a.Value.upperOpen {
		return a, nil
	}
	return b, nil
}
