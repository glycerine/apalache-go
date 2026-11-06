package realclosure

// Source: src/math/realclosure/realclosure.cpp:4516-4608, expanded Tarski
// query rows for a completed shared sign-determination result.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) expandCompletedSignQueries(root *algebraicRoot, q, q2 []*realValue, useSquare, usePrem bool) (queries []int, owned [][]*realValue, err error) {
	if root == nil || root.signs == nil || root.signs.deleted || len(root.signs.prs) != len(root.signs.tarski) {
		return nil, nil, arena.ErrInvalidSlot
	}
	step := 2
	if useSquare {
		step = 3
		if len(q2) == 0 {
			return nil, nil, arena.ErrInvalidSlot
		}
	}
	queries = make([]int, 0, len(root.signs.prs)*step)
	for i, row := range root.signs.prs {
		queries = append(queries, root.signs.tarski[i])
		product, e := s.multiplyAnyPolynomials(row.coefficients, q, usePrem)
		if e != nil {
			return nil, nil, errors.Join(e, s.releaseMultiRootProducts(owned))
		}
		owned = append(owned, product)
		query, e := s.tarskiBinaryIntervalAny(root.polynomial.coefficients, product, root.isolating, usePrem)
		if e != nil {
			return nil, nil, errors.Join(e, s.releaseMultiRootProducts(owned))
		}
		queries = append(queries, query)
		if useSquare {
			product, e = s.multiplyAnyPolynomials(row.coefficients, q2, usePrem)
			if e != nil {
				return nil, nil, errors.Join(e, s.releaseMultiRootProducts(owned))
			}
			owned = append(owned, product)
			query, e = s.tarskiBinaryIntervalAny(root.polynomial.coefficients, product, root.isolating, usePrem)
			if e != nil {
				return nil, nil, errors.Join(e, s.releaseMultiRootProducts(owned))
			}
			queries = append(queries, query)
		}
	}
	return queries, owned, nil
}

func (s *realValueStore) releaseMultiRootProducts(products [][]*realValue) (err error) {
	for _, product := range products {
		err = errors.Join(err, s.releaseSturmCoefficients(product))
	}
	return err
}
