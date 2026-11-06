package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *BoolPlugin) replaceAndOwn(target, value, formula ast.ASTHandle) (*BoolElimination, error) {
	result, err := p.elimination(formula, value)
	if err != nil {
		return nil, err
	}
	if err = p.replacer.ApplySubstitution(target, value, result.Formula); err != nil {
		return nil, errorsJoinBoolElimination(err, result)
	}
	return result, nil
}

func errorsJoinBoolElimination(cause error, result *BoolElimination) error {
	if result == nil {
		return cause
	}
	if closeErr := result.Close(); closeErr != nil {
		return errors.Join(cause, closeErr)
	}
	return cause
}

// Solve is the candidate-specific helper: it tries a unit for variable
// before polarity. SolveContext implements the complete source solve callback
// that selects among all context variables and publishes the elimination.
func (p *BoolPlugin) Solve(variable *ast.ContainsApp, formula ast.ASTHandle, conjs *ConjunctionEnum, atoms *BoolAtomIndex) (*BoolElimination, bool, error) {
	if err := p.checkInputs(variable, formula); err != nil {
		return nil, false, err
	}
	if atoms == nil || atoms.manager != p.manager || conjs == nil || conjs.manager != p.manager {
		return nil, false, ast.ErrASTHandle
	}
	result, solved, err := p.solveUnit(variable, formula, conjs)
	if err != nil || solved {
		return result, solved, err
	}
	return p.solvePolarized(variable, formula, atoms)
}
