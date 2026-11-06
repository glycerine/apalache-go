package model

// Source: src/tactic/filter_model_converter.cpp:31.

import "github.com/glycerine/g3/pkg/ast"

// NewFilteredModel acquires the model built by the retained filter converter
// before it copies unmarked interpretations and replaces the output reference.
func NewFilteredModel(m *ast.ASTManager) (*Model, error) {
	md, err := NewModel(m)
	if err != nil {
		return md, err
	}
	md.traceSourceAllocated("src/tactic/filter_model_converter.cpp", 31)
	return md, nil
}
