package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticPlugin) Substitute(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) error {
	if value == nil {
		return ast.ErrASTHandle
	}
	if err := p.open(); err != nil {
		return err
	}
	root, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if variable == nil {
		return ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return err
	}
	branchNumber, err := p.manager.NumeralService().Unsigned(value)
	if err != nil {
		return err
	}
	v := new(big.Rat).SetInt64(int64(branchNumber))
	b, err := p.branch(target, root, v)
	if err != nil {
		return err
	}
	result, err := p.manager.RefGet(b.Result)
	if err != nil {
		return err
	}
	if err = p.manager.SetRef(formula, result); err != nil {
		return err
	}
	if definition != nil {
		if b.Definition == (ast.ASTRef{}) {
			return p.manager.SetRef(*definition, ast.ASTHandle{})
		}
		d, err := p.manager.RefGet(b.Definition)
		if err != nil {
			return err
		}
		return p.manager.SetRef(*definition, d)
	}
	return nil
}

func (p *ArithmeticPlugin) Project(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef) (bool, error) {
	if err := p.Substitute(variable, value, formula, nil); err != nil {
		return false, err
	}
	return true, nil
}
