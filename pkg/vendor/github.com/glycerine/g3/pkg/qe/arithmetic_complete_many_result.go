package qe

import (
	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticCompleteProjectionResult struct {
	self                   *ArithmeticCompleteProjectionResult
	manager                *ast.ASTManager
	variables, definitions []ast.ASTRef
	closed                 bool
}

func newArithmeticCompleteProjectionResult(m *ast.ASTManager) *ArithmeticCompleteProjectionResult {
	r := &ArithmeticCompleteProjectionResult{manager: m}
	r.self = r
	return r
}
