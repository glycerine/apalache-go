package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) ProjectVariablesNNFComplete(variables []ast.ASTHandle, source *model.Model, formula ast.ASTRef, config Config) (result *ArithmeticCompleteProjectionResult, resultErr error) {
	if e := p.open(); e != nil {
		return nil, e
	}
	if e := p.validateCompleteMany(variables, source, formula); e != nil {
		return nil, e
	}
	result = newArithmeticCompleteProjectionResult(p.manager)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	for i := len(variables) - 1; i >= 0; i-- {
		if e := p.projectCompleteStep(variables[i], source, formula, config, result); e != nil {
			return nil, e
		}
	}
	return result, nil
}
