package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (s *ArithmeticAtomSet) Close() (result error) {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	for _, v := range []*[]ast.ASTRef{&s.negative, &s.positive} {
		for len(*v) > 0 {
			i := len(*v) - 1
			result = errors.Join(result, s.manager.DelRef(&(*v)[i]))
			if result != nil {
				return result
			}
			*v = (*v)[:i]
		}
	}
	s.closed = true
	return nil
}
