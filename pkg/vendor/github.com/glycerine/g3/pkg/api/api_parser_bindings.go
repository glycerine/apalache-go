package api

// Source: src/api/api_parsers.cpp:41-64.

import "github.com/glycerine/g3/pkg/ast"

type SMT2SortBinding struct {
	Name string
	Sort ast.ASTHandle
}

type SMT2DeclarationBinding struct {
	Name        string
	Declaration ast.ASTHandle
}

// SMT2Bindings preserves caller order and AST identity for names supplied to
// one parser invocation.
type SMT2Bindings struct {
	Sorts        []SMT2SortBinding
	Declarations []SMT2DeclarationBinding
}
