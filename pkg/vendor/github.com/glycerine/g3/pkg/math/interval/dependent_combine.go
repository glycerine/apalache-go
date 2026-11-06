package interval

// Source: src/math/interval/dep_intervals.h:53-80, 397-406.

func (m *DependentIntervalManager) combine(mask BoundDeps, first, second *DependentRationalInterval) *IntervalDependency {
	var result *IntervalDependency
	for _, item := range []struct {
		bit        BoundDeps
		dependency *IntervalDependency
	}{
		{DepLower1, first.LowerDep}, {DepLower2, second.LowerDep},
		{DepUpper1, first.UpperDep}, {DepUpper2, second.UpperDep},
	} {
		if mask&item.bit != 0 {
			result = m.Join(result, item.dependency)
		}
	}
	return result
}

func (m *DependentIntervalManager) publishBinaryDeps(first, second, result *DependentRationalInterval, deps IntervalDeps) {
	if result.Value.lowerInf {
		result.LowerDep = nil
	} else {
		result.LowerDep = m.combine(deps.Lower, first, second)
	}
	if result.Value.upperInf {
		result.UpperDep = nil
	} else {
		result.UpperDep = m.combine(deps.Upper, first, second)
	}
}
