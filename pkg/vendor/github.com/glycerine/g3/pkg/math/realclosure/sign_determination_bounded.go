package realclosure

// Source: src/math/realclosure/realclosure.cpp:1613-1741 and 1943-2093.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// countSignsAtZerosRationalOwned is count_signs_at_zeros with the source
// value_ref_buffer q2 lifetime transferred to its caller. The sign-
// determination loop reuses q2 in expand_taqrs before destroying it.
func (s *realValueStore) countSignsAtZerosRationalOwned(p, q []*realValue, interval *BinaryInterval, numRoots int, usePrem bool) (equal, positive, negative int, queriedSquare bool, q2 []*realValue, err error) {
	if s == nil || s.numbers == nil || interval == nil || numRoots <= 0 {
		return 0, 0, 0, false, nil, arena.ErrInvalidSlot
	}
	taqQ, err := s.tarskiBinaryIntervalRational(p, q, interval, usePrem)
	if err != nil {
		return 0, 0, 0, false, nil, err
	}
	// This is the condition on the source's second if/else. It is
	// deliberately not an else-if continuation of the +/-numRoots cases.
	queriedSquare = taqQ != numRoots-1 && taqQ != -(numRoots-1)
	if !queriedSquare {
		equal, positive, negative, _, err = countSignsFromTarski(numRoots, taqQ, 0)
		return equal, positive, negative, false, nil, err
	}
	q2, err = s.multiplyRationalPolynomials(q, q)
	if err != nil {
		return 0, 0, 0, true, nil, err
	}
	taqQ2, err := s.tarskiBinaryIntervalRational(p, q2, interval, usePrem)
	if err != nil {
		return 0, 0, 0, true, q2, err
	}
	equal, positive, negative, _, err = countSignsFromTarski(numRoots, taqQ, taqQ2)
	return equal, positive, negative, true, q2, err
}

func (s *signDetermination) releaseExpandedRationalProducts(owned [][]*realValue) (err error) {
	for _, values := range owned {
		err = errors.Join(err, s.store.releaseSturmCoefficients(values))
	}
	return err
}

// expandTarskiRational implements expand_taqrs for the concrete rational
// coefficient path. Products and their Tarski queries are generated in source
// row order: pr, pr*q, then (only for a three-column sign matrix) pr*q^2.
// expanded borrows the returned owned products until the caller transfers
// them into realPolynomial owners.
func (s *signDetermination) expandTarskiRational(p, q, q2 []*realValue, interval *BinaryInterval, useSquare, usePrem bool) (expandedTarski []int, expanded [][]*realValue, owned [][]*realValue, err error) {
	if s == nil || s.store == nil || interval == nil || len(s.prs) != len(s.tarski) || len(s.prs) != s.matrix.rows || useSquare && len(q2) == 0 {
		return nil, nil, nil, arena.ErrInvalidSlot
	}
	step := 2
	if useSquare {
		step = 3
	}
	expandedTarski = make([]int, 0, len(s.prs)*step)
	expanded = make([][]*realValue, 0, len(s.prs)*step)
	owned = make([][]*realValue, 0, len(s.prs)*(step-1))
	fail := func(body error) ([]int, [][]*realValue, [][]*realValue, error) {
		return nil, nil, nil, errors.Join(body, s.releaseExpandedRationalProducts(owned))
	}
	for i := range s.prs {
		prior := s.prs[i].coefficients
		expandedTarski = append(expandedTarski, s.tarski[i])
		expanded = append(expanded, prior)

		prq, productErr := s.store.multiplyRationalPolynomials(prior, q)
		if productErr != nil {
			return fail(productErr)
		}
		owned = append(owned, prq)
		query, queryErr := s.store.tarskiBinaryIntervalRational(p, prq, interval, usePrem)
		if queryErr != nil {
			return fail(queryErr)
		}
		expandedTarski = append(expandedTarski, query)
		expanded = append(expanded, prq)

		if useSquare {
			prq2, productErr := s.store.multiplyRationalPolynomials(prior, q2)
			if productErr != nil {
				return fail(productErr)
			}
			owned = append(owned, prq2)
			query, queryErr = s.store.tarskiBinaryIntervalRational(p, prq2, interval, usePrem)
			if queryErr != nil {
				return fail(queryErr)
			}
			expandedTarski = append(expandedTarski, query)
			expanded = append(expanded, prq2)
		}
	}
	return expandedTarski, expanded, owned, nil
}

// advanceBoundedRational executes one retained sign_det_isolate_roots loop
// iteration after the derivative q has been selected. It connects actual
// bounded Tarski queries and polynomial products to the existing exact matrix,
// cardinality, sign-condition and polynomial-owner transition.
func (s *signDetermination) advanceBoundedRational(p, q []*realValue, interval *BinaryInterval, usePrem bool) (kept, allOne bool, err error) {
	if s == nil || s.store == nil || interval == nil || len(s.prs) != s.matrix.rows || len(s.tarski) != s.matrix.rows || len(s.conditions) != s.matrix.cols {
		return false, false, arena.ErrInvalidSlot
	}
	equal, positive, negative, _, q2, bodyErr := s.store.countSignsAtZerosRationalOwned(p, q, interval, s.numRoots, usePrem)
	if bodyErr != nil {
		return false, false, errors.Join(bodyErr, s.store.releaseSturmCoefficients(q2))
	}
	qMatrix, useful := signDetMatrix(equal, positive, negative)
	if !useful {
		return false, false, s.store.releaseSturmCoefficients(q2)
	}
	useSquare := qMatrix.cols == 3
	expandedTarski, expanded, owned, bodyErr := s.expandTarskiRational(p, q, q2, interval, useSquare, usePrem)
	if bodyErr == nil {
		kept, allOne, bodyErr = s.advanceWithPolynomials(equal, positive, negative, q, expandedTarski, expanded)
	}
	productsErr := s.releaseExpandedRationalProducts(owned)
	squareErr := s.store.releaseSturmCoefficients(q2)
	return kept, allOne, errors.Join(bodyErr, productsErr, squareErr)
}
