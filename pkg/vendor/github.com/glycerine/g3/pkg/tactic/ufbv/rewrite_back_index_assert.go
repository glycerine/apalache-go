package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:482,507,518,607.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func assertBackIndexSetPresent(line int32) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil || !enabled {
		return err
	}
	return diagnostic.DebugAssertionFailure(debug, "src/tactic/ufbv/ufbv_rewriter.cpp", line, "it->m_value")
}

func (r *rewriteBackIndex) removeBackOccurrence(d, expr ast.ASTHandle) error {
	set, found := r.byDecl[d]
	if !found {
		return nil
	}
	if set == nil {
		if e := assertBackIndexSetPresent(507); e != nil {
			return e
		}
		return ast.ErrASTDomain
	}
	delete(set.values, expr)
	return nil
}
