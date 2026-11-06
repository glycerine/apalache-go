package qe

import "github.com/glycerine/g3/pkg/ast"

func (r *ArithmeticCompleteProjectionResult) validateDefinitionTarget(target *DefinitionVector) error {
	if e := r.open(); e != nil {
		return e
	}
	if target == nil || target.manager != r.manager {
		return ast.ErrASTHandle
	}
	return target.open()
}
