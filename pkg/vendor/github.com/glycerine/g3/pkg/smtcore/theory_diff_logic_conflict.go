package smtcore

// Source: src/smt/theory_diff_logic_def.h set_neg_cycle_conflict.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTRealDiffTheory) setNegativeCycleConflict(literals []SMTLiteral) (result error) {
	state, err := t.State()
	if err != nil {
		return err
	}
	var parameters []ast.ASTParameter
	proofs, err := state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		// Native uses one coefficient per antecedent and an additional unit
		// coefficient for the resulting contradiction.
		var antecedents smtArithmeticAntecedents
		for range literals {
			antecedents.literalCoefficients = append(antecedents.literalCoefficients, big.NewRat(1, 1))
		}
		parameters, err = antecedents.parameters(state.manager, "farkas")
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(state.manager, parameters)) }()
		one, err := state.manager.NewParameterRat(big.NewRat(1, 1))
		if err != nil {
			return err
		}
		parameters = append(parameters, one)
	}
	owner, err := NewSMTExtTheoryConflictJustification(state.manager, t.FamilyID(), literals, nil, parameters)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(state.manager))
	}
	state.ownedJustifications = append(state.ownedJustifications, owner)
	state.inconsistent = true
	state.conflict = wrapped
	state.conflictLiteral = SMTNullLiteral
	state.statistics.NumConflicts++
	t.realStatistics.conflicts++
	return nil
}
