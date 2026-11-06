package smtcore

// Source: src/smt/theory_arith_nl.h compute_grobner perturbation phase.

import "github.com/glycerine/g3/pkg/math/grobner"

// Each lower monomial in a linear equation can have its variable promoted
// once beyond the default weights 0..7. Reordering the basis changes its
// leading terms and permits another source basis-computation round.
func smtPerturbGrobnerOrder(gb *grobner.Solver, nextWeight *int) (bool, error) {
	for _, equation := range gb.Equations() {
		if equation.NumMonomials() == 0 || equation.Monomial(0).Degree() != 1 {
			continue
		}
		for i := uint32(1); i < equation.NumMonomials(); i++ {
			monomial := equation.Monomial(i)
			if monomial.Degree() != 1 {
				continue
			}
			variable := monomial.Variable(0)
			if gb.Weight(variable) > 7 {
				continue
			}
			if err := gb.SetWeight(variable, *nextWeight); err != nil {
				return false, err
			}
			*nextWeight++
			return true, gb.UpdateOrder()
		}
	}
	return false, nil
}
