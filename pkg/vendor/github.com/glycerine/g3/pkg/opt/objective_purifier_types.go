package opt

// Source: src/opt/opt_context.cpp purify/is_mul_const/mk_atomic.

import "github.com/glycerine/g3/pkg/ast"

type PurifiedObjectiveSet struct {
	Hard       []ast.ASTHandle
	Objectives []DecomposedObjective
}

type ObjectivePurificationStats struct {
	ArithmeticDefinitions uint32
	SoftDefinitions       uint32
	RebuiltSums           uint32
	FreshDeclarations     uint32
}
