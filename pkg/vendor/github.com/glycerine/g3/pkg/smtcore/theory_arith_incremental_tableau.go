package smtcore

import (
	"math/big"

	"github.com/glycerine/g3/pkg/math/simplex"
)

// Source theory_arith_core.h::assert_bound/propagate_core installs only the
// newly asserted bound into the live tableau.
func (t *SMTLinearArithmeticTheory) applyReplayAssertion(solver *simplex.Solver, atomBases []uint32, assertion smtIntegerDiffAssertion) (bool, error) {
	atom := &t.atoms[assertion.atom]
	if !atom.supported {
		return true, nil
	}
	lower, bound := smtLinearAtomBound(atom, assertion.truth, t.integralAtom(atom))
	base := atomBases[assertion.atom]
	dependency := simplex.BoundDependency{Literal: smtLinearAssertionLiteral(atom, assertion).Index(), Coefficient: *big.NewRat(1, 1)}
	solver.AddArithmeticBoundAtom(base, lower, bound, []simplex.BoundDependency{dependency})
	_, err := smtTightenExplainedSimplexBound(solver, base, lower, bound, []simplex.BoundDependency{dependency})
	if err != nil {
		t.recordArithmeticBoundConflict(solver, base, lower, &smtArithmeticBoundExplanation{dependencies: []simplex.BoundDependency{dependency}})
		return false, nil
	}
	// Source single-variable atoms constrain the variable directly. Keep
	// these bounds as well as the comparison row for nonlinear intervals.
	if len(atom.form.terms) == 1 {
		for variable, coefficient := range atom.form.terms {
			direct := bound.Clone()
			direct.Quo(coefficient)
			directLower := lower == (coefficient.Sign() > 0)
			directDependency := simplex.BoundDependency{Literal: dependency.Literal, Coefficient: *new(big.Rat).Inv(coefficient)}
			solver.AddArithmeticBoundAtom(variable, directLower, direct, []simplex.BoundDependency{directDependency})
			_, err = smtTightenExplainedSimplexBound(solver, variable, directLower, direct, []simplex.BoundDependency{directDependency})
			if err != nil {
				t.recordArithmeticBoundConflict(solver, variable, directLower, &smtArithmeticBoundExplanation{dependencies: []simplex.BoundDependency{directDependency}})
				return false, nil
			}
		}
	}
	return true, nil
}
