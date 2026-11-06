package realclosure

// Source: src/math/realclosure/realclosure.cpp:5658-5682.

import "errors"

func (s *realValueStore) powerRetainedValue(value *realValue, exponent uint32) (result *realValue, err error) {
	result, err = s.newRational(1, 1)
	if err != nil {
		return nil, err
	}
	if exponent == 0 {
		return result, nil
	}
	base := value
	if err = s.retain(base); err != nil {
		_ = s.releaseSturmScratch(result)
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(base)) }()
	for exponent != 0 {
		if err = s.checkpoint(); err != nil {
			_ = s.releaseSturmScratch(result)
			return nil, err
		}
		if exponent&1 != 0 {
			next, e := s.multiplyRetainedValues(result, base)
			if e != nil {
				_ = s.releaseSturmScratch(result)
				result = nil
				return nil, e
			}
			if e = s.releaseSturmScratch(result); e != nil {
				_ = s.releaseSturmScratch(next)
				result = nil
				return nil, e
			}
			result = next
		}
		exponent >>= 1
		if exponent == 0 {
			break
		}
		next, e := s.multiplyRetainedValues(base, base)
		if e != nil {
			return nil, e
		}
		if e = s.releaseSturmScratch(base); e != nil {
			_ = s.releaseSturmScratch(next)
			base = nil
			return nil, e
		}
		base = next
	}
	return result, nil
}
