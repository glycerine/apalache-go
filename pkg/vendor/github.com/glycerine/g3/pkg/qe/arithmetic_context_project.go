package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) ProjectContextVariablesNNFComplete(source *model.Model, formula ast.ASTRef, config Config) (*ArithmeticCompleteProjectionResult, error) {
	variables, e := p.contextVariables()
	if e != nil {
		return nil, e
	}
	if e = p.validateContextVariables(variables); e != nil {
		return nil, e
	}
	return p.ProjectVariablesNNFComplete(variables, source, formula, config)
}
