package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:411-421.

func (s *subsumptionSimplifier) clauseSubsumes(left, right *Clause) (bool, error) {
	if left.Size() > right.Size() || s.hashes[left]&s.hashes[right] != s.hashes[left] {
		return false, nil
	}
	for _, first := range left.Literals() {
		found := false
		for _, second := range right.Literals() {
			var err error
			found, err = s.literalSubsumes(first, second)
			if err != nil {
				return false, err
			}
			if found {
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}
