package realclosure

// Source: src/math/realclosure/realclosure.cpp:3399-3413.

func (s *realValueStore) numSignConditions(value *realValue) (int, error) {
	condition, _, err := s.signConditionAt(value, 0)
	if err != nil {
		return 0, err
	}
	count := 0
	for condition != nil {
		count++
		condition = condition.previous
	}
	return count, nil
}
