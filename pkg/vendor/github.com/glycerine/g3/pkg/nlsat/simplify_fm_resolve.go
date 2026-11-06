package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:640-677.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) resolveFourierMotzkin(clauses []*Clause, lo, hi []*fourierMotzkinBound) (err error) {
	core := s.atoms.ineq.core
	for _, lower := range lo {
		la, lb, cause := lower.values()
		if cause != nil {
			return cause
		}
		for _, upper := range hi {
			ua, ub, cause := upper.values()
			if cause != nil {
				return cause
			}
			left, cause := polynomial.NewPolynomialRef(core)
			if cause != nil {
				return cause
			}
			right, cause := polynomial.NewPolynomialRef(core)
			if cause != nil {
				_ = left.Close()
				return cause
			}
			result, cause := polynomial.NewPolynomialRef(core)
			if cause != nil {
				_ = left.Close()
				_ = right.Close()
				return cause
			}
			product, cause := core.Mul(lb, ua)
			if cause == nil {
				cause = left.Assign(product)
			}
			if cause == nil {
				product, cause = core.Mul(ub, la)
			}
			if cause == nil {
				cause = right.Assign(product)
			}
			var p, leftValue, rightValue polynomial.Polynomial
			if cause == nil {
				leftValue, cause = left.Get()
			}
			if cause == nil {
				rightValue, cause = right.Get()
			}
			if cause == nil {
				p, cause = core.Sub(leftValue, rightValue)
			}
			if cause == nil {
				cause = result.Assign(p)
			}
			if cause == nil {
				p, cause = result.Get()
			}
			strict := lower.strict || upper.strict
			var literal = FalseLiteral
			if cause == nil {
				literal, cause = s.fourierMotzkinLiteral(p, strict)
			}
			cause = errors.Join(cause, result.Close(), right.Close(), left.Close())
			if cause != nil {
				return cause
			}
			if literal == TrueLiteral {
				continue
			}
			assumptions, cause := s.assumptions.mkJoin(lower.clause.Assumptions(), upper.clause.Assumptions())
			if cause != nil {
				return cause
			}
			if _, cause = s.clauses.mkClause([]sat.Literal{literal}, false, assumptions); cause != nil {
				return cause
			}
		}
	}
	seen := make(map[*Clause]bool)
	for _, clause := range clauses {
		if seen[clause] || s.clauses.owned[clause] == nil {
			continue
		}
		seen[clause] = true
		if err = s.clauses.deleteClause(clause); err != nil {
			return err
		}
	}
	return nil
}
