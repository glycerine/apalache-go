package nlsat

// Source: src/nlsat/nlsat_explain.cpp:1694-1741.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// signedPolynomialOwners retains every arithmetic result across the complete
// virtual-substitution calculation. Polynomial operations may recycle cache
// entries, so a raw result is never carried across the next operation.
type signedPolynomialOwners struct {
	core *polynomial.MultivariatePolynomialCore
	refs []*polynomial.PolynomialRef
}

func (o *signedPolynomialOwners) keep(value polynomial.Polynomial, cause error) (polynomial.Polynomial, error) {
	if cause != nil {
		return polynomial.Polynomial{}, cause
	}
	ref, err := polynomial.NewPolynomialRef(o.core)
	if err != nil {
		return polynomial.Polynomial{}, err
	}
	o.refs = append(o.refs, ref)
	if err = ref.Assign(value); err != nil {
		return polynomial.Polynomial{}, err
	}
	return ref.Get()
}

func (o *signedPolynomialOwners) close() (err error) {
	for i := len(o.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, o.refs[i].Close())
	}
	o.refs = nil
	return err
}

func (o *signedPolynomialOwners) constant(value int32) (polynomial.Polynomial, error) {
	n := unumeral.NewInt(value)
	return o.keep(o.core.MkConstantTakeCopy(&n))
}

// signedSolveLinearEquality applies ax+b=0 as x=B/A, where B=-b. For every
// other polynomial q of degree d it constructs
//
//	A^d*q(B/A)
//
// in the source coefficient order and records its model sign. The equality's
// leading coefficient sign is recorded last at its original vector position.
func (n *explanationAssumptions) signedSolveLinearEquality(x, equalityIndex uint32, ps *polynomial.PolynomialRefVector) (err error) {
	if ps == nil {
		return arena.ErrConfig
	}
	core := n.search.store.atoms.ineq.core
	count, err := ps.Size()
	if err != nil {
		return err
	}
	if equalityIndex >= count {
		return arena.ErrBounds
	}
	equality, err := ps.Get(equalityIndex)
	if err != nil {
		return err
	}
	degree, err := core.Degree(equality, x)
	if err != nil {
		return err
	}
	if err = atomAssert("nlsat_explain.cpp", 1701, "degree(p, x) == 1", degree == 1); err != nil {
		return err
	}
	if degree != 1 {
		return arena.ErrConfig
	}
	owners := signedPolynomialOwners{core: core}
	defer func() { err = errors.Join(err, owners.close()) }()
	a, err := owners.keep(core.Coeff(equality, x, 1))
	if err != nil {
		return err
	}
	b0, err := owners.keep(core.Coeff(equality, x, 0))
	if err != nil {
		return err
	}
	b, err := owners.keep(core.Neg(b0))
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		if i == equalityIndex {
			if _, err = n.ensurePolynomialSign(a); err != nil {
				return err
			}
			continue
		}
		q, err := ps.Get(i)
		if err != nil {
			return err
		}
		d, err := core.Degree(q, x)
		if err != nil {
			return err
		}
		one, err := owners.constant(1)
		if err != nil {
			return err
		}
		powers := make([]polynomial.Polynomial, 0, d+1)
		power := one
		for j := uint32(0); j <= d; j++ {
			powers = append(powers, power)
			power, err = owners.keep(core.Mul(power, a))
			if err != nil {
				return err
			}
		}
		e, err := owners.constant(1)
		if err != nil {
			return err
		}
		r, err := owners.constant(0)
		if err != nil {
			return err
		}
		for j := uint32(0); j <= d; j++ {
			coefficient, err := owners.keep(core.Coeff(q, x, j))
			if err != nil {
				return err
			}
			zero, err := core.IsZero(coefficient)
			if err != nil {
				return err
			}
			if !zero {
				term, cause := owners.keep(core.Mul(powers[d-j], e))
				if cause != nil {
					return cause
				}
				term, cause = owners.keep(core.Mul(term, coefficient))
				if cause != nil {
					return cause
				}
				r, cause = owners.keep(core.Add(r, term))
				if cause != nil {
					return cause
				}
			}
			e, err = owners.keep(core.Mul(e, b))
			if err != nil {
				return err
			}
		}
		if _, err = n.ensurePolynomialSign(r); err != nil {
			return err
		}
	}
	return nil
}
