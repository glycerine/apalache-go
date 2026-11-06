package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) solveEquation(conjunctions *ConjunctionEnum, formula ast.ASTHandle) (*ArithmeticEquationElimination, bool, error) {
	if conjunctions == nil || conjunctions.manager != p.manager {
		return nil, false, ast.ErrASTHandle
	}
	count, err := p.context.NumVariables()
	if err != nil {
		return nil, false, err
	}
	variables := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		variables[index], err = p.context.Variable(index)
		if err != nil {
			return nil, false, err
		}
	}
	equalities, err := conjunctions.ExtractEqualities()
	if err != nil {
		return nil, false, err
	}
	defer func() {
		for index := len(equalities) - 1; index >= 0; index-- {
			_ = p.manager.DelRef(&equalities[index])
		}
	}()
	for _, equality := range equalities {
		expression, cause := p.manager.RefGet(equality)
		if cause != nil {
			return nil, false, cause
		}
		equation, cause := p.parseEquation(expression, variables)
		if cause != nil {
			return nil, false, cause
		}
		result, solved, cause := p.solveSingularEquation(equation, variables, formula)
		if cause != nil || solved {
			return result, solved, cause
		}
		result, solved, cause = p.solveLinearIntegerEquation(equation, variables, formula)
		if cause != nil || solved {
			return result, solved, cause
		}
	}
	return nil, false, nil
}

func (p *ArithmeticPlugin) discardPendingEquation() error {
	if p.pendingEquation == nil {
		return nil
	}
	err := p.pendingEquation.Close()
	if err == nil {
		p.pendingEquation = nil
	}
	return err
}

func (p *ArithmeticPlugin) TakeEquationElimination() (*ArithmeticEquationElimination, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := p.pendingEquation
	p.pendingEquation = nil
	return result, nil
}
