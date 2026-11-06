package api

// Source: src/api/api_opt.cpp:215-224,240-251.

import "github.com/glycerine/g3/pkg/ast"

func (o *APIOptimize) Upper(index uint32) (ast.ASTRef, error) {
	return o.objectiveBound(index, false)
}

func (o *APIOptimize) UpperVector(index uint32) ([]ast.ASTRef, error) {
	return o.objectiveBoundVector(index, false)
}
