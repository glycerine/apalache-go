package nlsat

// Source: src/nlsat/nlsat_explain.cpp:1465-1529.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func literalContainsVariable(variables []uint32, variable uint32) bool {
	for _, current := range variables {
		if current == variable {
			return true
		}
	}
	return false
}

// SetSignedProjection selects the source explain::set_signed_project mode.
// General projection is the default; NLQSAT enables signed projection.
func (s *Solver) SetSignedProjection(enabled bool) error {
	if err := s.open(); err != nil {
		return err
	}
	s.explanation.signedProjection = enabled
	return nil
}

// Project eliminates one arithmetic variable from literals that are true in
// the current model. Result owns the returned literals and is reset first.
func (s *Solver) Project(variable uint32, literals []sat.Literal, result *LiteralVector) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if result == nil || result.solver != s {
		return ErrSearchOwner
	}
	if err = result.owner(); err != nil {
		return err
	}
	if uint64(variable) >= uint64(len(s.clauses.vars.isInt)) {
		return arena.ErrBounds
	}
	if err = result.values.reset(); err != nil {
		return err
	}
	if err = s.explanation.reset(); err != nil {
		return err
	}

	dependent := make([]sat.Literal, 0, len(literals))
	independent := make([]sat.Literal, 0, len(literals))
	variables := make([]uint32, 0, 4)
	for _, literal := range literals {
		value, cause := s.LiteralValue(literal)
		if cause != nil {
			return cause
		}
		if value != lbool.True {
			return arena.ErrConfig
		}
		if cause = s.LiteralVariables(literal, &variables); cause != nil {
			return cause
		}
		if literalContainsVariable(variables, variable) {
			dependent = append(dependent, literal)
		} else {
			independent = append(independent, literal.Negated())
		}
	}

	ps, err := polynomial.NewPolynomialRefVector(s.search.store.atoms.ineq.core)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ps.Close()) }()
	if err = s.explanation.collectPolynomials(dependent, ps); err != nil {
		return err
	}
	maximum, err := s.explanation.maxPolynomialVariable(ps)
	if err != nil && len(dependent) != 0 {
		return err
	}

	reordered := false
	if len(dependent) != 0 && variable != maximum {
		permutation := make([]uint32, len(s.clauses.vars.isInt))
		for i := range permutation {
			permutation[i] = uint32(i)
		}
		permutation[variable], permutation[maximum] = permutation[maximum], permutation[variable]
		if err = s.ReorderVariables(permutation); err != nil {
			return err
		}
		reordered = true
	}
	defer func() {
		if reordered {
			err = errors.Join(err, s.RestoreVariableOrder())
		}
	}()

	n := s.explanation
	if err = n.start(); err != nil {
		return err
	}
	for _, literal := range independent {
		if err = n.addLiteral(literal); err != nil {
			err = n.finish(err)
			err = errors.Join(err, n.cleanup())
			return err
		}
	}
	if len(dependent) != 0 {
		if err = n.elimVanishingVector(ps); err == nil {
			if n.signedProjection {
				err = n.signedProject(ps, maximum)
			} else {
				err = n.project(ps, maximum)
			}
		}
	}
	if err = n.finish(err); err != nil {
		err = errors.Join(err, n.cleanup())
		return err
	}

	// Restore before publishing: projected atoms borrow polynomials renamed by
	// the temporary order and must be observed in the external variable order.
	if reordered {
		if err = s.RestoreVariableOrder(); err != nil {
			return err
		}
		reordered = false
	}
	for _, literal := range n.output.result {
		if err = result.values.push(literal.Negated()); err != nil {
			return err
		}
	}
	return n.reset()
}
