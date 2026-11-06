package realclosure

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *signDetermination) expandTarskiAny(p, q, q2 []*realValue, interval *BinaryInterval, useSquare, usePrem bool) (queries []int, expanded [][]*realValue, owned [][]*realValue, err error) {
	if s == nil || s.store == nil || interval == nil || len(s.prs) != len(s.tarski) || len(s.prs) != s.matrix.rows || useSquare && len(q2) == 0 {
		return nil, nil, nil, arena.ErrInvalidSlot
	}
	step := 2
	if useSquare {
		step = 3
	}
	queries = make([]int, 0, len(s.prs)*step)
	expanded = make([][]*realValue, 0, len(s.prs)*step)
	owned = make([][]*realValue, 0, len(s.prs)*(step-1))
	fail := func(cause error) ([]int, [][]*realValue, [][]*realValue, error) {
		return nil, nil, nil, errors.Join(cause, s.store.releaseExpandedAnyProducts(owned))
	}
	for i := range s.prs {
		prior := s.prs[i].coefficients
		queries = append(queries, s.tarski[i])
		expanded = append(expanded, prior)
		product, e := s.store.multiplyAnyPolynomials(prior, q, usePrem)
		if e != nil {
			return fail(e)
		}
		owned = append(owned, product)
		query, e := s.store.tarskiBinaryIntervalAny(p, product, interval, usePrem)
		if e != nil {
			return fail(e)
		}
		queries = append(queries, query)
		expanded = append(expanded, product)
		if useSquare {
			product, e = s.store.multiplyAnyPolynomials(prior, q2, usePrem)
			if e != nil {
				return fail(e)
			}
			owned = append(owned, product)
			query, e = s.store.tarskiBinaryIntervalAny(p, product, interval, usePrem)
			if e != nil {
				return fail(e)
			}
			queries = append(queries, query)
			expanded = append(expanded, product)
		}
	}
	return queries, expanded, owned, nil
}

func (s *realValueStore) releaseExpandedAnyProducts(owned [][]*realValue) (err error) {
	for _, values := range owned {
		err = errors.Join(err, s.releaseSturmCoefficients(values))
	}
	return err
}

func (s *signDetermination) advanceBoundedAny(p, q []*realValue, interval *BinaryInterval, usePrem bool) (kept, allOne bool, err error) {
	if s == nil || s.store == nil || interval == nil || len(s.prs) != s.matrix.rows || len(s.tarski) != s.matrix.rows || len(s.conditions) != s.matrix.cols {
		return false, false, arena.ErrInvalidSlot
	}
	equal, positive, negative, _, q2, bodyErr := s.store.countSignsAtZerosAnyOwned(p, q, interval, s.numRoots, usePrem)
	if bodyErr != nil {
		return false, false, errors.Join(bodyErr, s.store.releaseSturmCoefficients(q2))
	}
	qMatrix, useful := signDetMatrix(equal, positive, negative)
	if !useful {
		return false, false, s.store.releaseSturmCoefficients(q2)
	}
	queries, expanded, owned, bodyErr := s.expandTarskiAny(p, q, q2, interval, qMatrix.cols == 3, usePrem)
	if bodyErr == nil {
		kept, allOne, bodyErr = s.advanceWithPolynomials(equal, positive, negative, q, queries, expanded)
	}
	productsErr := s.store.releaseExpandedAnyProducts(owned)
	squareErr := s.store.releaseSturmCoefficients(q2)
	return kept, allOne, errors.Join(bodyErr, productsErr, squareErr)
}
