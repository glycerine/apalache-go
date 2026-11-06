package qe

// Source: src/qe/qe_bv_plugin.cpp subst.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *BitVectorPlugin) Substitute(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) error {
	if value == nil {
		return ast.ErrASTHandle
	}
	current, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if err = p.checkInputs(variable, current); err != nil {
		return err
	}
	target, err := variable.Target()
	if err != nil {
		return err
	}
	width, err := p.manager.BitVectorSize(target)
	if err != nil {
		return err
	}
	replacement, err := p.manager.BitVectorNumeral(value, width)
	if err != nil {
		return err
	}
	if err = p.replacer.ApplySubstitution(target, replacement, formula); err != nil {
		return err
	}
	if definition != nil {
		realizer, makeErr := p.manager.BitVectorNumeral(value, width)
		if makeErr != nil {
			return makeErr
		}
		if err = p.manager.SetRef(*definition, realizer); err != nil {
			return err
		}
	}
	return nil
}
