package api

// Source: src/api/api_opt.cpp:360-373, Z3_optimize_get_assertions.

import "github.com/glycerine/g3/pkg/ast"

// AssertionRefs returns independently owning references. It is the owning
// counterpart of Assertions, retained under a distinct name for Go API
// compatibility with the earlier borrowed-handle query.
func (o *APIOptimize) AssertionRefs() ([]ast.ASTRef, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.command.OptimizationAssertions()
}
