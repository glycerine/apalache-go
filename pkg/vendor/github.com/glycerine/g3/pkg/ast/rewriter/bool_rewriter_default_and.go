package rewriter

// Source: src/ast/rewriter/bool_rewriter.h:81,135-140;
// src/model/model2expr.cpp:76,153.

import "github.com/glycerine/g3/pkg/ast"

// DefaultAnd performs the source stack-local bool_rewriter(m).mk_and call.
// Its default parameters enable flattening and disable AND elimination and
// local-context rewriting. The temporary borrows m and owns no persistent
// configuration or vector storage.
func DefaultAnd(m *ast.ASTManager, args []ast.ASTHandle, out ast.ASTRef) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	rw := boolRewriter{manager: m}
	status, e := rw.mkFlatAndCore(args, out)
	if e != nil || status != brFailed {
		return e
	}
	h, e := m.MkAndArgs(args)
	if e != nil {
		return e
	}
	return m.SetRef(out, h)
}
