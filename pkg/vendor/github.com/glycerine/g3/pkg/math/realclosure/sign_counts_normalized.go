package realclosure

// signCountsUnboundedNormalized computes TaQ(q) and TaQ(q²) over the roots
// of p, then applies count_signs_at_zeros_core.
func (s *realValueStore) signCountsUnboundedNormalized(p, q []*realValue) (equal, positive, negative int, err error) {
	numRoots, err := s.rootCountUnboundedNormalized(p)
	if err != nil || numRoots == 0 {
		return 0, 0, 0, err
	}
	taq, err := s.tarskiUnboundedNormalized(p, q)
	if err != nil {
		return 0, 0, 0, err
	}
	square, err := s.multiplyNormalizedPolynomials(q, q)
	if err != nil {
		return 0, 0, 0, err
	}
	taqSquare, err := s.tarskiUnboundedNormalized(p, square)
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
