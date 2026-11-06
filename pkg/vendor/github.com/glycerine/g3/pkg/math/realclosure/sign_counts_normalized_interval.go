package realclosure

func (s *realValueStore) signCountsIntervalNormalized(p, q []*realValue, interval *BinaryInterval) (equal, positive, negative int, err error) {
	numRoots, err := s.rootCountIntervalNormalized(p, interval)
	if err != nil || numRoots == 0 {
		return 0, 0, 0, err
	}
	taq, err := s.tarskiIntervalNormalized(p, q, interval)
	if err != nil {
		return 0, 0, 0, err
	}
	square, err := s.multiplyNormalizedPolynomials(q, q)
	if err != nil {
		return 0, 0, 0, err
	}
	taqSquare, err := s.tarskiIntervalNormalized(p, square, interval)
	releaseErr := s.releaseTemporaryCoefficients(square)
	if err != nil {
		return 0, 0, 0, err
	}
	if releaseErr != nil {
		return 0, 0, 0, releaseErr
	}
	equal, positive, negative, _, err = countSignsFromTarski(numRoots, taq, taqSquare)
	return equal, positive, negative, err
}
