package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *BoolPlugin) solveUnit(variable *ast.ContainsApp, formula ast.ASTHandle, conjs *ConjunctionEnum) (*BoolElimination, bool, error) {
	if conjs == nil {
		return nil, false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return nil, false, err
	}
	count, err := conjs.Len()
	if err != nil {
		return nil, false, err
	}
	for index := uint32(0); index < count; index++ {
		current, cause := conjs.At(index)
		if cause != nil {
			return nil, false, cause
		}
		value, matched, cause := p.unitValue(current, target)
		if cause != nil {
			return nil, false, cause
		}
		if !matched {
			continue
		}
		result, cause := p.replaceAndOwn(target, value, formula)
		return result, cause == nil, cause
	}
	return nil, false, nil
}

func (p *BoolPlugin) unitValue(expression, target ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if expression == target {
		value, err := p.manager.MkTrue()
		return value, err == nil, err
	}
	kindOfNode, err := p.manager.GetKind(expression)
	if err != nil || kindOfNode != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	kind, err := p.manager.GetDeclKind(expression)
	if err != nil || family != p.manager.BasicFamilyID() || kind != ast.ASTOpNot {
		return ast.ASTHandle{}, false, err
	}
	argument, err := p.manager.GetArg(expression, 0)
	if err != nil || argument != target {
		return ast.ASTHandle{}, false, err
	}
	value, err := p.manager.MkFalse()
	return value, err == nil, err
}
