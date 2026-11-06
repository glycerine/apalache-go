package rewriter

import "github.com/glycerine/g3/pkg/ast"

// VarShifter exposes the concrete source var_shifter to retained consumers
// outside the AST rewriter package, including UFBV substitution under binders.
type VarShifter = variableShifter

func NewVarShifter(m *ast.ASTManager) (*VarShifter, error) {
	return newVarShifter(m)
}

// NewInverseVarShifter exposes the existing concrete inverse specialization
// to retained QE callers that remove freshly substituted binder levels.
func NewInverseVarShifter(m *ast.ASTManager) (*VarShifter, error) {
	return newInvVarShifter(m)
}
