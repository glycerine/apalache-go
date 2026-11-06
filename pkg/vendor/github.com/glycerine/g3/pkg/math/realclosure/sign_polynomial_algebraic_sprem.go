package realclosure

// signedPseudoRemainderNormalized is sprem plus the source integer-content
// normalization used by the default real-closure Sturm path.
func (s *realValueStore) signedPseudoRemainderNormalized(left, right []*realValue) ([]*realValue, error) {
	result, steps, err := s.pseudoRemainderNormalized(left, right)
	if err != nil || len(result) == 0 {
		return result, err
	}
	leadingSign, err := s.signValue(right[len(right)-1])
	if err != nil {
		_ = s.releaseTemporaryCoefficients(result)
		return nil, err
	}
	if steps%2 == 0 || leadingSign > 0 {
		for i, value := range result {
			negative, cause := s.negateValue(value)
			if cause != nil {
				_ = s.releaseTemporaryCoefficients(result)
				return nil, cause
			}
			if cause = s.release(value); cause != nil {
				_ = s.release(negative)
				_ = s.releaseTemporaryCoefficients(result)
				return nil, cause
			}
			result[i] = negative
		}
	}
	if err = s.normalizeIntegerContentNormalized(result); err != nil {
		_ = s.releaseTemporaryCoefficients(result)
		return nil, err
	}
	return trimRealCoefficients(result), nil
}
