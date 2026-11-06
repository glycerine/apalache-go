package qe

// Source: src/qe/qe_arith_plugin.cpp branch_formula key.

import "github.com/glycerine/g3/pkg/ast"

type ArithmeticBranchKey struct {
	Formula  ast.ASTHandle
	Variable ast.ASTHandle
	Branch   uint32
}
