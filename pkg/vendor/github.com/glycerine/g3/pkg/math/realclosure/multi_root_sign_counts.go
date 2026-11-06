package realclosure

// Source: src/math/realclosure/realclosure.cpp:4460-4515, count the signs of
// q at every defining-polynomial root in a shared isolating interval.

import "github.com/glycerine/g3/pkg/arena"

type multiRootSignCounts struct {
	equal, positive, negative int
	useSquare                 bool
	square                    []*realValue
}

func (s *realValueStore) countMultiRootPolynomialSigns(root *algebraicRoot, q []*realValue, usePrem bool) (multiRootSignCounts, error) {
	if root == nil || root.deleted || root.signs == nil || root.isolating == nil {
		return multiRootSignCounts{}, arena.ErrInvalidSlot
	}
	numRoots, err := root.numRootsInsideInterval()
	if err != nil {
		return multiRootSignCounts{}, err
	}
	equal, positive, negative, useSquare, square, err := s.countSignsAtZerosAnyOwned(root.polynomial.coefficients, q, root.isolating, numRoots, usePrem)
	return multiRootSignCounts{equal: equal, positive: positive, negative: negative, useSquare: useSquare, square: square}, err
}
