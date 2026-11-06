package smtcore

// Source: src/smt/theory_arith_nl.h is_perfect_square overloads and
// is_inconsistent2, which recognizes expanded (M1-M2)^2 equations.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/grobner"
	"github.com/glycerine/g3/pkg/math/simplex"
)

func smtGrobnerPerfectSquare(monomial *grobner.Monomial) (*big.Rat, bool) {
	if monomial.Degree()%2 != 0 {
		return nil, false
	}
	coefficient := monomial.Coefficient()
	if coefficient.Sign() < 0 {
		return nil, false
	}
	numerator := new(big.Int).Sqrt(coefficient.Num())
	denominator := new(big.Int).Sqrt(coefficient.Denom())
	if new(big.Int).Mul(numerator, numerator).Cmp(coefficient.Num()) != 0 || new(big.Int).Mul(denominator, denominator).Cmp(coefficient.Denom()) != 0 {
		return nil, false
	}
	for i := uint32(0); i < monomial.Degree(); i += 2 {
		if monomial.Variable(i) != monomial.Variable(i+1) {
			return nil, false
		}
	}
	return new(big.Rat).SetFrac(numerator, denominator), true
}

func smtGrobnerSquareCross(first *grobner.Monomial, a *big.Rat, second *grobner.Monomial, b *big.Rat, cross *grobner.Monomial) bool {
	coefficient := cross.Coefficient()
	if coefficient.Sign() >= 0 {
		return false
	}
	expected := new(big.Rat).Mul(a, b)
	expected.Mul(expected, big.NewRat(-2, 1))
	if coefficient.Cmp(expected) != 0 || uint64(first.Degree())+uint64(second.Degree()) != 2*uint64(cross.Degree()) {
		return false
	}
	i, j := uint32(0), uint32(0)
	for k := uint32(0); k < cross.Degree(); k++ {
		variable := cross.Variable(k)
		if i < first.Degree() && first.Variable(i) == variable {
			i += 2
		} else if j < second.Degree() && second.Variable(j) == variable {
			j += 2
		} else {
			return false
		}
	}
	return i == first.Degree() && j == second.Degree()
}

func (t *SMTLinearArithmeticTheory) grobnerMonomialInterval(solver *simplex.Solver, columns map[ast.ASTHandle]uint32, manager *smtArithmeticBoundDependencyManager, monomial *grobner.Monomial) (*smtOldInterval, error) {
	interval, err := newSMTOldIntervalPoint(manager, monomial.Coefficient(), nil, nil)
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < monomial.Degree(); {
		h := monomial.Variable(i)
		j := i + 1
		for j < monomial.Degree() && monomial.Variable(j) == h {
			j++
		}
		column, found := columns[h]
		if !found {
			return nil, ast.ErrASTDomain
		}
		if err = t.multiplyNonlinearColumnBound(solver, column, j-i, interval); err != nil {
			return nil, err
		}
		i = j
	}
	return interval, nil
}

// Source is_inconsistent(I, monomials, dep) copies I and stops once
// its sum is fully unbounded. Later monomials cannot exclude zero and
// must not perform additional interval work or consume resources.
func (t *SMTLinearArithmeticTheory) grobnerSumInterval(solver *simplex.Solver, columns map[ast.ASTHandle]uint32, manager *smtArithmeticBoundDependencyManager, initial *smtOldInterval, monomials []*grobner.Monomial) (*smtOldInterval, error) {
	sum := initial.clone()
	for _, monomial := range monomials {
		interval, err := t.grobnerMonomialInterval(solver, columns, manager, monomial)
		if err != nil {
			return nil, err
		}
		if err = sum.add(interval); err != nil {
			return nil, err
		}
		if sum.minusInfinity() && sum.plusInfinity() {
			break
		}
	}
	return sum, nil
}

func (t *SMTLinearArithmeticTheory) grobnerSquareConflictInterval(solver *simplex.Solver, columns map[ast.ASTHandle]uint32, manager *smtArithmeticBoundDependencyManager, monomials []*grobner.Monomial) (*smtOldInterval, error) {
	intervals := make([]*smtOldInterval, len(monomials))
	for i, monomial := range monomials {
		interval, err := t.grobnerMonomialInterval(solver, columns, manager, monomial)
		if err != nil {
			return nil, err
		}
		intervals[i] = interval
	}
	deleted := make([]bool, len(monomials))
	removed := false
	for i, first := range monomials {
		if deleted[i] {
			continue
		}
		a, square := smtGrobnerPerfectSquare(first)
		if !square {
			continue
		}
		found := false
		for j := i + 1; j < len(monomials) && !found; j++ {
			if deleted[j] {
				continue
			}
			second := monomials[j]
			b, square := smtGrobnerPerfectSquare(second)
			if !square {
				continue
			}
			for k := i + 1; k < len(monomials); k++ {
				if deleted[k] || !smtGrobnerSquareCross(first, a, second, b, monomials[k]) {
					continue
				}
				interval := intervals[i].clone()
				if err := interval.add(intervals[j]); err != nil {
					return nil, err
				}
				if err := interval.add(intervals[k]); err != nil {
					return nil, err
				}
				// Keep tighter existing intervals. Replace a recognized
				// square only when [0,+infinity) improves its lower bound.
				if interval.minusInfinity() || interval.lower.sign() < 0 {
					deleted[i], deleted[j], deleted[k] = true, true, true
					removed, found = true, true
					break
				}
			}
		}
	}
	if !removed {
		return nil, nil
	}
	sum, err := newSMTOldIntervalHalfBound(manager, new(big.Rat), false, true, nil)
	if err != nil {
		return nil, err
	}
	var remaining []*grobner.Monomial
	for i, monomial := range monomials {
		if !deleted[i] {
			remaining = append(remaining, monomial)
		}
	}
	// Source invokes is_inconsistent again after selecting squares. This
	// reevaluates the surviving monomials, including their resource charges.
	sum, err = t.grobnerSumInterval(solver, columns, manager, sum, remaining)
	if err != nil {
		return nil, err
	}
	if sum.containsZero() {
		return nil, nil
	}
	return sum, nil
}
