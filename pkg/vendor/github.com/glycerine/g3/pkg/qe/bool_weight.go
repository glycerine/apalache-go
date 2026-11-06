package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *BoolPlugin) Weight(variable *ast.ContainsApp, atoms *BoolAtomIndex) (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	if variable == nil || atoms == nil || atoms.manager != p.manager {
		return 0, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return 0, err
	}
	positive, err := atoms.Positive(target)
	if err != nil {
		return 0, err
	}
	negative, err := atoms.Negative(target)
	if err != nil {
		return 0, err
	}
	if positive && negative {
		return 3, nil
	}
	return 0, nil
}
