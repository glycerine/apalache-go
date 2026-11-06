package api

// Source: src/api/api_opt.cpp:204-213,226-238.

import "github.com/glycerine/g3/pkg/ast"

func (o *APIOptimize) Lower(index uint32) (ast.ASTRef, error) {
	return o.objectiveBound(index, true)
}

func (o *APIOptimize) LowerVector(index uint32) ([]ast.ASTRef, error) {
	return o.objectiveBoundVector(index, true)
}
