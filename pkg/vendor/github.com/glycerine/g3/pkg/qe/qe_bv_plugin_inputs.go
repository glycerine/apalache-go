package qe

// Source: src/qe/qe_bv_plugin.cpp reached contains_app and formula contracts.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *BitVectorPlugin) checkInputs(variable *ast.ContainsApp, formula ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if variable == nil {
		return ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return err
	}
	if kind, cause := p.manager.GetKind(target); cause != nil || kind != ast.ASTApp {
		return errors.Join(ast.ErrASTDomain, cause)
	}
	bitVector, err := p.manager.IsBitVector(target)
	if err != nil || !bitVector {
		return errors.Join(ast.ErrASTDomain, err)
	}
	if _, err = p.manager.GetKind(formula); err != nil {
		return err
	}
	return nil
}
