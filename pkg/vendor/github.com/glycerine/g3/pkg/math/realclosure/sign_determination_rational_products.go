package realclosure

// Source: src/math/realclosure/realclosure.cpp:1704-1741 and 2018-2028.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// advanceRationalProducts builds the retained prs*1, prs*q, and optional
// prs*q² rows before the Tarski/cardinality transition. The still-unported
// Tarski query supplies the result for each row in that source order.
func (s *signDetermination) advanceRationalProducts(equal, positive, negative int, q []*realValue, expandedTarski []int) (bool, bool, error) {
	if s == nil || s.store == nil || len(s.prs) != s.matrix.rows {
		return false, false, arena.ErrInvalidSlot
	}
	qMatrix, usefulSigns := signDetMatrix(equal, positive, negative)
	if !usefulSigns {
		return false, false, nil
	}
	if len(expandedTarski) != len(s.prs)*qMatrix.rows {
		return false, false, arena.ErrInvalidSlot
	}
	owned := make([][]*realValue, 0, len(s.prs)*qMatrix.cols)
	cleanup := func() error {
		var err error
		for _, values := range owned {
			err = errors.Join(err, s.store.releaseCoefficients(values))
		}
		return err
	}
	var square []*realValue
	if qMatrix.cols == 3 {
		var err error
		square, err = s.store.multiplyRationalPolynomials(q, q)
		if err != nil {
			return false, false, err
		}
		owned = append(owned, square)
	}
	expanded := make([][]*realValue, 0, len(s.prs)*qMatrix.cols)
	for _, prior := range s.prs {
		expanded = append(expanded, prior.coefficients)
		product, err := s.store.multiplyRationalPolynomials(prior.coefficients, q)
		if err != nil {
			return false, false, errors.Join(err, cleanup())
		}
		owned = append(owned, product)
		expanded = append(expanded, product)
		if qMatrix.cols == 3 {
			product, err = s.store.multiplyRationalPolynomials(prior.coefficients, square)
			if err != nil {
				return false, false, errors.Join(err, cleanup())
			}
			owned = append(owned, product)
			expanded = append(expanded, product)
		}
	}
	kept, allOne, err := s.advanceWithPolynomials(equal, positive, negative, q, expandedTarski, expanded)
	return kept, allOne, errors.Join(err, cleanup())
}
