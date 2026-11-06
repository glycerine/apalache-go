package realclosure

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// advanceAlgebraicProducts is the source prs*1/prs*q/prs*q² expansion once
// normalize_algebraic is available for the q² and prior*q products.
func (s *signDetermination) advanceAlgebraicProducts(equal, positive, negative int, q []*realValue, expandedTarski []int) (bool, bool, error) {
	if s == nil || s.store == nil || len(s.prs) != s.matrix.rows {
		return false, false, arena.ErrInvalidSlot
	}
	qMatrix, useful := signDetMatrix(equal, positive, negative)
	if !useful {
		return false, false, nil
	}
	if len(expandedTarski) != len(s.prs)*qMatrix.rows {
		return false, false, arena.ErrInvalidSlot
	}
	owned := make([][]*realValue, 0, len(s.prs)*qMatrix.cols)
	cleanup := func() error {
		var err error
		for _, values := range owned {
			err = errors.Join(err, s.store.releaseTemporaryCoefficients(values))
		}
		return err
	}
	var square []*realValue
	if qMatrix.cols == 3 {
		var err error
		square, err = s.store.multiplyNormalizedPolynomials(q, q)
		if err != nil {
			return false, false, err
		}
		owned = append(owned, square)
	}
	expanded := make([][]*realValue, 0, len(s.prs)*qMatrix.cols)
	for _, prior := range s.prs {
		expanded = append(expanded, prior.coefficients)
		product, err := s.store.multiplyNormalizedPolynomials(prior.coefficients, q)
		if err != nil {
			return false, false, errors.Join(err, cleanup())
		}
		owned = append(owned, product)
		expanded = append(expanded, product)
		if qMatrix.cols == 3 {
			product, err = s.store.multiplyNormalizedPolynomials(prior.coefficients, square)
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
