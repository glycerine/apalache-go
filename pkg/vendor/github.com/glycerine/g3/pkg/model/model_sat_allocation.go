package model

// Source: src/sat/tactic/sat_tactic.cpp:89-112.

import "github.com/glycerine/g3/pkg/ast"

// NewSATBooleanModel acquires the model used by the retained SAT Boolean
// registration fragment before it publishes any declaration bindings.
func NewSATBooleanModel(m *ast.ASTManager) (*Model, error) {
	md, err := NewModel(m)
	if err != nil {
		return md, err
	}
	md.traceSourceAllocated("src/sat/tactic/sat_tactic.cpp", 93)
	return md, nil
}
