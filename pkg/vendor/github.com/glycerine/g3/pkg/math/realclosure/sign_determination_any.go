package realclosure

import "github.com/glycerine/g3/pkg/arena"

// countSignsAtZerosAnyOwned is the source count_signs_at_zeros flow for
// retained coefficients that may be rational functions or algebraic values.
// q2's ownership passes to the caller when the source queries it.
func (s *realValueStore) countSignsAtZerosAnyOwned(p, q []*realValue, interval *BinaryInterval, numRoots int, usePrem bool) (equal, positive, negative int, queriedSquare bool, q2 []*realValue, err error) {
	if s == nil || s.numbers == nil || interval == nil || numRoots <= 0 {
		return 0, 0, 0, false, nil, arena.ErrInvalidSlot
	}
	if err = s.checkAnyPolynomial(p, false); err != nil {
		return 0, 0, 0, false, nil, err
	}
	if err = s.checkAnyPolynomial(q, true); err != nil {
		return 0, 0, 0, false, nil, err
	}
	if err = interval.open(); err != nil {
		return 0, 0, 0, false, nil, err
	}
	taqQ, err := s.tarskiBinaryIntervalAny(p, q, interval, usePrem)
	if err != nil {
		return 0, 0, 0, false, nil, err
	}
	queriedSquare = taqQ != numRoots-1 && taqQ != -(numRoots-1)
	if !queriedSquare {
		equal, positive, negative, _, err = countSignsFromTarski(numRoots, taqQ, 0)
		return equal, positive, negative, false, nil, err
	}
	q2, err = s.multiplyAnyPolynomials(q, q, usePrem)
	if err != nil {
		return 0, 0, 0, true, nil, err
	}
	taqQ2, err := s.tarskiBinaryIntervalAny(p, q2, interval, usePrem)
	if err != nil {
		return 0, 0, 0, true, q2, err
	}
	equal, positive, negative, _, err = countSignsFromTarski(numRoots, taqQ, taqQ2)
	return equal, positive, negative, true, q2, err
}
