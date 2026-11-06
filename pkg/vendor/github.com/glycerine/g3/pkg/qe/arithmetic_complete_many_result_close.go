package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (r *ArithmeticCompleteProjectionResult) Close() (result error) {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	for _, v := range []*[]ast.ASTRef{&r.definitions, &r.variables} {
		for len(*v) > 0 {
			i := len(*v) - 1
			result = errors.Join(result, r.manager.DelRef(&(*v)[i]))
			if result != nil {
				return result
			}
			*v = (*v)[:i]
		}
	}
	r.closed = true
	return nil
}
