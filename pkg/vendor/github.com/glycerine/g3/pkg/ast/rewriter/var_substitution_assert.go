package rewriter

// Source: src/ast/rewriter/var_subst.cpp:27,34; src/util/debug.h:42-56.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *VarSubstitution) assertWellSorted(h ast.ASTHandle, line int32) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil || !enabled {
		return err
	}
	good, err := s.reducer.engine.core.manager.IsWellSorted(h, nil)
	if err != nil || good {
		return err
	}
	condition := "is_well_sorted(result.m(), n)"
	if line == 34 {
		condition = "is_well_sorted(m_reducer.m(), result)"
	}
	return diagnostic.DebugAssertionFailure(debug, "src/ast/rewriter/var_subst.cpp", line, condition)
}

func (s *VarSubstitution) assertResultWellSorted(out ast.ASTRef) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil || !enabled {
		return err
	}
	h, err := s.reducer.engine.core.manager.RefGet(out)
	if err != nil {
		return err
	}
	return s.assertWellSorted(h, 34)
}
