package qe

import (
	"github.com/glycerine/g3/pkg/ast"
)

// ArithmeticAtomSet owns source-ordered positive and negative atom roots.
type ArithmeticAtomSet struct {
	self                       *ArithmeticAtomSet
	manager                    *ast.ASTManager
	positive, negative         []ast.ASTRef
	positiveSeen, negativeSeen map[ast.ASTHandle]bool
	closed                     bool
}

func NewArithmeticAtomSet(manager *ast.ASTManager) (*ArithmeticAtomSet, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	s := &ArithmeticAtomSet{manager: manager, positiveSeen: make(map[ast.ASTHandle]bool), negativeSeen: make(map[ast.ASTHandle]bool)}
	s.self = s
	return s, nil
}
