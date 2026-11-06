package qe

// Source: src/qe/qe_bv_plugin.cpp is_uninterpreted/get_weight and the inherited
// qe_solver_plugin atom callback.

import "github.com/glycerine/g3/pkg/ast"

func (p *BitVectorPlugin) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	return ast.ASTHandle{}, false, nil
}

func (p *BitVectorPlugin) IsUninterpreted(application ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	kind, err := p.manager.GetKind(application)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	family, err := p.manager.GetFamilyID(application)
	if err != nil || family != p.family {
		return false, err
	}
	declarationKind, err := p.manager.GetDeclKind(application)
	if err != nil {
		return false, err
	}
	return declarationKind >= 12 && declarationKind <= 21, nil
}

func (p *BitVectorPlugin) Weight(variable *ast.ContainsApp, formula ast.ASTHandle) (uint32, error) {
	if err := p.checkInputs(variable, formula); err != nil {
		return 0, err
	}
	return 2, nil
}

func (p *BitVectorPlugin) Solve(*ConjunctionEnum, ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return false, nil
}
