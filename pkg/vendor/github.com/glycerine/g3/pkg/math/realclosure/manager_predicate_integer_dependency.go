package realclosure

// Source: src/math/realclosure/realclosure.cpp:2464-2497, 2528-2533,
// 6274-6292.

func (s *realValueStore) queryValueIsInteger(value *realValue) (bool, error) {
	return s.isIntegerValue(value)
}

func (s *realValueStore) queryValueDependsOnInfinitesimals(value *realValue) (bool, error) {
	return s.dependsOnInfinitesimalsValue(value)
}
