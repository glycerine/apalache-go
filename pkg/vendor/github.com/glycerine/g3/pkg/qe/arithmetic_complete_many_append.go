package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (r *ArithmeticCompleteProjectionResult) append(variable ast.ASTHandle, definition ast.ASTRef) (result error) {
	v, e := r.manager.NewRef(variable)
	if e != nil {
		return e
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, r.manager.DelRef(&v))
		}
	}()
	r.variables = append(r.variables, v)
	r.definitions = append(r.definitions, definition)
	return nil
}
