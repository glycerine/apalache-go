package nlsat

// Source: src/nlsat/nlsat_explain.cpp:1517-1529,1763-1783,1830-1832;
// src/nlsat/nlsat_explain.h:90-102.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

// splitLiteralsForVariable preserves source input order. Literals independent
// of variable are ignored by maximize; dependent literals are returned for
// polynomial collection.
func (s *Solver) splitLiteralsForVariable(variable uint32, literals []sat.Literal) ([]sat.Literal, error) {
	dependent := make([]sat.Literal, 0, len(literals))
	variables := make([]uint32, 0, 4)
	for _, literal := range literals {
		if err := s.LiteralVariables(literal, &variables); err != nil {
			return nil, err
		}
		if literalContainsVariable(variables, variable) {
			dependent = append(dependent, literal)
		}
	}
	return dependent, nil
}

// Maximize computes the source's local upper boundary for variable under the
// current assignment to all other variables. It selects the least real root
// of the dependent conflict polynomials that is greater than or equal to the
// current value. If none exists, unbounded is true and out is unchanged.
func (s *Solver) Maximize(variable uint32, literals []sat.Literal, out *polynomial.Algebraic) (unbounded bool, err error) {
	if err = s.open(); err != nil {
		return false, err
	}
	if out == nil {
		return false, polynomial.ErrAlgebraicOwner
	}
	if uint64(variable) >= uint64(len(s.clauses.vars.isInt)) {
		return false, arena.ErrBounds
	}
	assigned, err := s.assignment.Contains(variable)
	if err != nil {
		return false, err
	}
	if !assigned {
		return false, arena.ErrConfig
	}
	dependent, err := s.splitLiteralsForVariable(variable, literals)
	if err != nil {
		return false, err
	}
	polynomials, err := polynomial.NewPolynomialRefVector(s.search.store.atoms.ineq.core)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, polynomials.Close()) }()
	if err = s.explanation.collectPolynomials(dependent, polynomials); err != nil {
		return false, err
	}
	roots, err := s.config.Algebraic.NewVector()
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, roots.Close()) }()
	current, err := s.assignment.Value(variable)
	if err != nil {
		return false, err
	}
	unbounded = true
	count, err := polynomials.Size()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		p, err := polynomials.Get(i)
		if err != nil {
			return false, err
		}
		if err = roots.Reset(); err != nil {
			return false, err
		}
		if err = s.evaluator.isolateRoots(p, variable, roots); err != nil {
			return false, err
		}
		for j := uint32(0); j < roots.Len(); j++ {
			root, err := roots.Borrow(j)
			if err != nil {
				return false, err
			}
			order, err := s.config.Algebraic.Compare(current, root)
			if err != nil {
				return false, err
			}
			if order > 0 {
				continue
			}
			better := unbounded
			if !unbounded {
				order, err = s.config.Algebraic.Compare(root, out)
				if err != nil {
					return false, err
				}
				better = order <= 0
			}
			if better {
				if err = s.config.Algebraic.Copy(out, root); err != nil {
					return false, err
				}
				unbounded = false
			}
		}
	}
	return unbounded, nil
}
