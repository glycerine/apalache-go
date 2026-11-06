package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:679-727.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) substituteFourierMotzkinLiteral(x uint32, numerator, denominator polynomial.Polynomial, literal sat.Literal) (result sat.Literal, err error) {
	raw, err := s.clauses.literalAtom(literal)
	if err != nil || raw == nil {
		return literal, err
	}
	atom, ok := raw.(*InequalityAtom)
	if !ok {
		return literal, nil
	}
	core := s.atoms.ineq.core
	refs := make([]*polynomial.PolynomialRef, 0, atom.Size())
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, refs[i].Close())
		}
	}()
	values := make([]polynomial.Polynomial, 0, atom.Size())
	even := make([]bool, 0, atom.Size())
	kind, changed := atom.kind, false
	for i := uint32(0); i < atom.Size(); i++ {
		old, cause := atom.Polynomial(i)
		if cause != nil {
			return result, cause
		}
		ref, cause := polynomial.NewPolynomialRef(core)
		if cause != nil {
			return result, cause
		}
		refs = append(refs, ref)
		if cause = core.SubstitutePolynomialFraction(old, x, numerator, denominator, ref); cause != nil {
			return result, cause
		}
		value, cause := ref.Get()
		if cause != nil {
			return result, cause
		}
		equal, cause := core.Equal(old, value)
		if cause != nil {
			return result, cause
		}
		changed = changed || !equal
		zero, cause := core.IsZero(value)
		if cause != nil {
			return result, cause
		}
		if zero {
			values = []polynomial.Polynomial{value}
			even = []bool{false}
			break
		}
		constant, cause := core.IsConstant(value)
		if cause != nil {
			return result, cause
		}
		parity, cause := atom.IsEven(i)
		if cause != nil {
			return result, cause
		}
		if constant {
			coefficient, cause := core.Coefficient(value, 0)
			if cause != nil {
				return result, cause
			}
			z, cause := core.IntegerManager()
			if cause != nil {
				return result, cause
			}
			negative, cause := z.IsNeg(&coefficient)
			if cause != nil {
				return result, cause
			}
			if negative && !parity {
				kind, cause = kind.Flip()
				if cause != nil {
					return result, cause
				}
			}
			continue
		}
		values, even = append(values, value), append(even, parity)
	}
	if !changed {
		return literal, nil
	}
	if len(values) == 0 {
		result, err = s.atoms.mkInequalityLiteral(kind, nil, nil)
	} else if len(values) == 1 {
		result, err = s.fourierMotzkinSubstitutedLiteral(kind, values[0], even[0])
	} else {
		result, err = s.MkInequality(kind, values, even, false)
	}
	if err == nil && literal.Sign() {
		result = result.Negated()
	}
	return result, err
}
