package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticPlugin) completeBranchValue(value *numeral.OwnedRational) (uint32, error) {
	if value == nil {
		return 0, ast.ErrASTHandle
	}
	v, e := p.manager.NumeralService().Unsigned(value)
	return v, e
}
