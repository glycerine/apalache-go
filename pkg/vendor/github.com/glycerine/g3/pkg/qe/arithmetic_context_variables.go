package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) contextVariables() ([]ast.ASTHandle, error) {
	n, e := p.context.NumVariables()
	if e != nil {
		return nil, e
	}
	result := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		result[i], e = p.context.Variable(i)
		if e != nil {
			return nil, e
		}
	}
	return result, nil
}
