package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *BoolPlugin) solvePolarized(variable *ast.ContainsApp, formula ast.ASTHandle, atoms *BoolAtomIndex) (*BoolElimination, bool, error) {
	target, err := variable.Target()
	if err != nil {
		return nil, false, err
	}
	positive, err := atoms.Positive(target)
	if err != nil {
		return nil, false, err
	}
	negative, err := atoms.Negative(target)
	if err != nil {
		return nil, false, err
	}
	if err := p.traceBoolPolarity(target, formula); err != nil {
		return nil, false, err
	}
	if positive && negative {
		return nil, false, nil
	}
	if positive {
		if clear, cause := p.onlyTargetOccurrence(variable, target, atoms.positiveAtoms(), atoms.negativeAtoms()); cause != nil || !clear {
			return nil, false, cause
		}
		truth, cause := p.manager.MkTrue()
		if cause != nil {
			return nil, false, cause
		}
		result, cause := p.replaceAndOwn(target, truth, formula)
		return result, cause == nil, cause
	}
	if negative {
		if clear, cause := p.onlyTargetOccurrence(variable, target, atoms.negativeAtoms(), atoms.positiveAtoms()); cause != nil || !clear {
			return nil, false, cause
		}
		falsity, cause := p.manager.MkFalse()
		if cause != nil {
			return nil, false, cause
		}
		result, cause := p.replaceAndOwn(target, falsity, formula)
		return result, cause == nil, cause
	}
	found, err := variable.Check(formula)
	if err != nil || found {
		return nil, false, err
	}
	falsity, err := p.manager.MkFalse()
	if err != nil {
		return nil, false, err
	}
	result, err := p.elimination(formula, falsity)
	return result, err == nil, err
}

func (p *BoolPlugin) onlyTargetOccurrence(variable *ast.ContainsApp, target ast.ASTHandle, selected, opposite map[ast.ASTHandle]ast.ASTRef) (bool, error) {
	for atom := range selected {
		if atom == target {
			continue
		}
		found, err := variable.Check(atom)
		if err != nil || found {
			return false, err
		}
	}
	for atom := range opposite {
		found, err := variable.Check(atom)
		if err != nil || found {
			return false, err
		}
	}
	return true, nil
}
