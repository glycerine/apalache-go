package qe

import "github.com/glycerine/g3/pkg/ast"

type arithmeticAtomVisit struct {
	expression ast.ASTHandle
	positive   bool
}
type arithmeticAtomCollector struct {
	manager *ast.ASTManager
	result  *ArithmeticAtomSet
	seen    map[arithmeticAtomVisit]bool
}
