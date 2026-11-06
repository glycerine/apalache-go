package realclosure

// Source: src/math/realclosure/realclosure.cpp:3415-3438.

func (s *realValueStore) signConditionSign(value *realValue, index int) (int, error) {
	condition, _, err := s.signConditionAt(value, index)
	if err != nil || condition == nil {
		return 0, err
	}
	return condition.sign, nil
}
