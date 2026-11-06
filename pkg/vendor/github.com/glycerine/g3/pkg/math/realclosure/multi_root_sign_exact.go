package realclosure

// Source: src/math/realclosure/realclosure.cpp:4460-4745.

import "errors"

func (s *realValueStore) exactMultiRootPolynomialSign(value *realValue, usePrem bool) (sign int32, err error) {
	root := value.root
	counts, err := s.countMultiRootPolynomialSigns(root, value.functionNumerator.coefficients, usePrem)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(counts.square)) }()
	expanded, products, err := s.expandCompletedSignQueries(root, value.functionNumerator.coefficients, counts.square, counts.useSquare, usePrem)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseMultiRootProducts(products)) }()
	_, cardinalities, qMatrix, err := solveCompletedSignExtension(root.signs, counts, expanded)
	if err != nil {
		return 0, err
	}
	return selectExtendedRootSign(root, cardinalities, &qMatrix)
}
