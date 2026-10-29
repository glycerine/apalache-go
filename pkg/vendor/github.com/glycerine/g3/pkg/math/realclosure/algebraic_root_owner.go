package realclosure

// Source: src/math/realclosure/realclosure.cpp:233-299, 825-856,
// and 1808-1835.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// algebraicRootOwner is the concrete algebraic-extension registry. Indices
// remain stable after an extension is deleted, matching m_extensions.
type algebraicRootOwner struct {
	values    *realValueStore
	intervals *numeral.BinaryRationalManager
	roots     []*algebraicRoot
}

type algebraicRoot struct {
	owner          *algebraicRootOwner
	index          int
	refs           uint32
	deleted        bool
	polynomial     realPolynomial
	interval       *BinaryInterval
	isolating      *BinaryInterval
	signs          *signDet
	signIndex      int
	infinitesimals bool
}

// mkAlgebraic constructs the retained algebraic extension. The source
// extension begins at reference count zero; a rational-function value will
// retain it when that value is published.
func (o *algebraicRootOwner) mkAlgebraic(coefficients []*realValue, enclosure, isolating *BinaryInterval, signs *signDet, signIndex int) (*algebraicRoot, error) {
	if o == nil || o.values == nil || o.intervals == nil || enclosure == nil || isolating == nil || (signs != nil && (signs.store != o.values || signs.deleted || signIndex < 0 || signIndex >= len(signs.conditions))) || (signs == nil && signIndex != -1) {
		return nil, arena.ErrInvalidSlot
	}
	r := &algebraicRoot{owner: o, index: len(o.roots), signIndex: signIndex}
	o.roots = append(o.roots, r)
	cleanup := func(err error) (*algebraicRoot, error) {
		_ = r.destroy()
		o.roots[r.index] = nil
		return nil, err
	}
	if err := o.values.setPolynomial(&r.polynomial, coefficients); err != nil {
		return cleanup(err)
	}
	var err error
	r.interval, err = NewBinaryInterval(o.intervals)
	if err != nil {
		return cleanup(err)
	}
	if err = r.interval.SetFrom(enclosure); err != nil {
		return cleanup(err)
	}
	r.isolating, err = NewBinaryInterval(o.intervals)
	if err != nil {
		return cleanup(err)
	}
	if err = r.isolating.SetFrom(isolating); err != nil {
		return cleanup(err)
	}
	if signs != nil {
		if err = signs.retain(); err != nil {
			return cleanup(err)
		}
		r.signs = signs
	}
	// All currently constructed coefficient values are rational; their
	// depends-on-infinitesimals predicate is false.
	return r, nil
}

func (r *algebraicRoot) retain() error {
	if r == nil || r.deleted || r.owner == nil || r.owner.roots[r.index] != r || r.refs == ^uint32(0) {
		return arena.ErrInvalidSlot
	}
	r.refs++
	return nil
}

func (r *algebraicRoot) release() error {
	if r == nil || r.deleted || r.owner == nil || r.owner.roots[r.index] != r || r.refs == 0 {
		return arena.ErrInvalidSlot
	}
	if r.refs > 1 {
		r.refs--
		return nil
	}
	if err := r.destroy(); err != nil {
		return err
	}
	r.refs = 0
	r.deleted = true
	r.owner.roots[r.index] = nil
	return nil
}

// destroy follows del_algebraic: polynomial, approximation interval,
// isolating interval, then the shared sign-determination owner.
func (r *algebraicRoot) destroy() error {
	if err := r.owner.values.resetPolynomial(&r.polynomial); err != nil {
		return err
	}
	if r.interval != nil {
		if err := r.interval.Close(); err != nil {
			return err
		}
		r.interval = nil
	}
	if r.isolating != nil {
		if err := r.isolating.Close(); err != nil {
			return err
		}
		r.isolating = nil
	}
	if r.signs != nil {
		if err := r.signs.release(); err != nil {
			return err
		}
		r.signs = nil
	}
	return nil
}

func (r *algebraicRoot) numRootsInsideInterval() (int, error) {
	if r == nil || r.deleted || r.owner == nil || r.owner.roots[r.index] != r {
		return 0, arena.ErrInvalidSlot
	}
	if r.signs == nil {
		return 1, nil
	}
	return len(r.signs.prs), nil
}
