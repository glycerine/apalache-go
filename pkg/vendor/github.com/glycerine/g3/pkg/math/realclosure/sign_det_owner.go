package realclosure

// Source: src/math/realclosure/realclosure.cpp:254-281, 788-824,
// and 1780-1807.

import "github.com/glycerine/g3/pkg/arena"

// signDet is the concrete array-owning result shared by algebraic roots.
// Its reference count starts at zero, as in mk_sign_det; each published root
// retains it once before the source sign-determination scope exits.
type signDet struct {
	store      *realValueStore
	refs       uint32
	deleted    bool
	matrix     signMatrix
	prs, qs    []realPolynomial
	tarski     []int
	conditions []*signCondition
}

// finish copies the prs/qs polynomial arrays, then transfers the matrix and
// sign-condition columns. The working scope releases its own polynomial refs.
func (s *signDetermination) finish() (*signDet, error) {
	if s == nil || s.store == nil || s.matrix.rows != s.numRoots || s.matrix.cols != s.numRoots || len(s.prs) != s.matrix.rows || len(s.tarski) != s.matrix.rows || len(s.conditions) != s.matrix.cols || len(s.qs) != s.polynomials {
		return nil, arena.ErrInvalidSlot
	}
	d := &signDet{store: s.store, prs: make([]realPolynomial, len(s.prs)), qs: make([]realPolynomial, len(s.qs))}
	for i := range s.prs {
		if err := d.store.setPolynomial(&d.prs[i], s.prs[i].coefficients); err != nil {
			_ = d.dispose()
			return nil, err
		}
	}
	for i := range s.qs {
		if err := d.store.setPolynomial(&d.qs[i], s.qs[i].coefficients); err != nil {
			_ = d.dispose()
			return nil, err
		}
	}
	d.tarski = append([]int(nil), s.tarski...)
	if err := s.closePolynomials(); err != nil {
		_ = d.dispose()
		return nil, err
	}
	d.matrix, s.matrix = s.matrix, signMatrix{}
	d.conditions, s.conditions = s.conditions, nil
	s.tarski = nil
	s.store = nil
	return d, nil
}

func (d *signDet) retain() error {
	if d == nil || d.store == nil || d.deleted || d.refs == ^uint32(0) {
		return arena.ErrInvalidSlot
	}
	d.refs++
	return nil
}

func (d *signDet) release() error {
	if d == nil || d.store == nil || d.deleted || d.refs == 0 {
		return arena.ErrInvalidSlot
	}
	if d.refs > 1 {
		d.refs--
		return nil
	}
	if err := d.dispose(); err != nil {
		return err
	}
	d.refs = 0
	return nil
}

// dispose follows del_sign_det: matrix, distinct sign-condition nodes, prs,
// Tarski results, qs. Native Go slices replace the array allocation headers.
func (d *signDet) dispose() error {
	d.matrix = signMatrix{}
	for _, head := range d.conditions {
		for current := head; current != nil && !current.deleted; {
			previous := current.previous
			current.deleted = true
			current.previous = nil
			current = previous
		}
	}
	d.conditions = nil
	for i := range d.prs {
		if err := d.store.resetPolynomial(&d.prs[i]); err != nil {
			return err
		}
	}
	d.prs = nil
	d.tarski = nil
	for i := range d.qs {
		if err := d.store.resetPolynomial(&d.qs[i]); err != nil {
			return err
		}
	}
	d.qs = nil
	d.deleted = true
	return nil
}
