package realclosure

// Source: src/math/realclosure/realclosure.cpp:5057-5085.

import "errors"

func (s *realValueStore) subtractRetainedValues(left, right *realValue) (result *realValue, err error) {
	negative, err := s.negateRetainedValue(right)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(negative)) }()
	return s.addRetainedValues(left, negative)
}
