package qe

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func (s *ArithmeticAtomSet) open() error {
	if s == nil || s.self != s || s.manager == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
