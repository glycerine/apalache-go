package qe

// Source: src/qe/qe_bv_plugin.cpp assign/get_num_branches.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *BitVectorPlugin) GetNumBranches(variable *ast.ContainsApp, formula ast.ASTHandle, branches *numeral.OwnedRational) (bool, error) {
	if branches == nil {
		return false, ast.ErrASTHandle
	}
	if err := p.checkInputs(variable, formula); err != nil {
		return false, err
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	width, err := p.manager.BitVectorSize(target)
	if err != nil {
		return false, err
	}
	count := new(big.Int).Lsh(big.NewInt(1), uint(width))
	if err = p.manager.NumeralService().SetString(branches, count.String()); err != nil {
		return false, err
	}
	return true, nil
}

// Assign is intentionally empty in the source BV plugin.
func (p *BitVectorPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, _ *numeral.OwnedRational) error {
	return p.checkInputs(variable, formula)
}
