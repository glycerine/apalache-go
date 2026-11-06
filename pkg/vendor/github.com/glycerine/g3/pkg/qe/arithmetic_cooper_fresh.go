package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticPlugin) TakeFreshVariables() ([]ast.ASTRef, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := p.pendingFresh
	p.pendingFresh = nil
	return result, nil
}

func (p *ArithmeticPlugin) clearPendingFresh() (result error) {
	for i := len(p.pendingFresh) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.pendingFresh[i]))
	}
	p.pendingFresh = nil
	return result
}
