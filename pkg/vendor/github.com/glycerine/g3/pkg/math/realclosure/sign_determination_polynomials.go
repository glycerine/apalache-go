package realclosure

// Source: src/math/realclosure/realclosure.cpp:778-808, 1780-1804,
// and 2050-2093.

import "github.com/glycerine/g3/pkg/arena"

// attachInitialPolynomial establishes prs = [1] for the source's initial
// one-by-one sign matrix. The caller retains ownership of its input value.
func (s *signDetermination) attachInitialPolynomial(store *realValueStore, one *realValue) error {
	if s == nil || store == nil || s.store != nil || s.polynomials != 0 || s.matrix.rows != 1 || one == nil {
		return arena.ErrInvalidSlot
	}
	var initial realPolynomial
	if err := store.setPolynomial(&initial, []*realValue{one}); err != nil {
		return err
	}
	s.store = store
	s.prs = []realPolynomial{initial}
	return nil
}

// advanceWithPolynomials adds the source's polynomial rows and q alongside
// the matrix transition. expandedPrs is the concrete result of multiplying
// each previous row by 1, q, and when needed q²; that multiplication and the
// Tarski-query calculation are still owned by the unfinished value port.
func (s *signDetermination) advanceWithPolynomials(equal, positive, negative int, q []*realValue, expandedTarski []int, expandedPrs [][]*realValue) (bool, bool, error) {
	if s == nil || s.store == nil || len(s.prs) != s.matrix.rows {
		return false, false, arena.ErrInvalidSlot
	}
	qMatrix, usefulSigns := signDetMatrix(equal, positive, negative)
	if !usefulSigns {
		return false, false, nil
	}
	if len(expandedPrs) != s.matrix.rows*qMatrix.rows {
		return false, false, arena.ErrInvalidSlot
	}
	var newQ realPolynomial
	if err := s.store.setPolynomial(&newQ, q); err != nil {
		return false, false, err
	}
	newPrs := make([]realPolynomial, len(expandedPrs))
	for i, coefficients := range expandedPrs {
		if err := s.store.setPolynomial(&newPrs[i], coefficients); err != nil {
			for j := 0; j < i; j++ {
				_ = s.store.resetPolynomial(&newPrs[j])
			}
			_ = s.store.resetPolynomial(&newQ)
			return false, false, err
		}
	}
	kept, allOne, rows := s.advanceCore(equal, positive, negative, expandedTarski)
	if !kept {
		for i := range newPrs {
			if err := s.store.resetPolynomial(&newPrs[i]); err != nil {
				return false, false, err
			}
		}
		if err := s.store.resetPolynomial(&newQ); err != nil {
			return false, false, err
		}
		return false, false, nil
	}
	selected := make([]realPolynomial, len(rows))
	for i, row := range rows {
		selected[i] = newPrs[row]
		newPrs[row] = realPolynomial{}
	}
	for i := range newPrs {
		if err := s.store.resetPolynomial(&newPrs[i]); err != nil {
			return false, false, err
		}
	}
	for i := range s.prs {
		if err := s.store.resetPolynomial(&s.prs[i]); err != nil {
			return false, false, err
		}
	}
	s.prs = selected
	s.qs = append(s.qs, newQ)
	return true, allOne, nil
}

// closePolynomials follows del_sign_det's polynomial array order: prs then qs.
func (s *signDetermination) closePolynomials() error {
	if s == nil || s.store == nil {
		return arena.ErrInvalidSlot
	}
	for i := range s.prs {
		if err := s.store.resetPolynomial(&s.prs[i]); err != nil {
			return err
		}
	}
	s.prs = nil
	for i := range s.qs {
		if err := s.store.resetPolynomial(&s.qs[i]); err != nil {
			return err
		}
	}
	s.qs = nil
	return nil
}
