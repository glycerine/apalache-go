package model

// Source: src/tactic/model_converter.cpp:176-181.

import "github.com/glycerine/g3/pkg/ast"

// NewConverterModel acquires the derived model allocated by
// model_converter2model before its model_ref assignment and callback.
func NewConverterModel(m *ast.ASTManager) (*Model, error) {
	md, err := NewModel(m)
	if err != nil {
		return md, err
	}
	md.traceSourceAllocated("src/tactic/model_converter.cpp", 178)
	return md, nil
}
