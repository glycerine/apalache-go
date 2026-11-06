package qe

// Source: src/qe/qe.h i_expr_pred/i_nnf_atom and src/qe/qe.cpp nnf.

import "github.com/glycerine/g3/pkg/ast"

type QENNFRelevance interface {
	IsRelevant(ast.ASTHandle) (bool, error)
}

type QENNFAtomNormalizer interface {
	NormalizeAtom(ast.ASTHandle, bool) (ast.ASTHandle, error)
}

type qeNNFKey struct {
	expression ast.ASTHandle
	positive   bool
}
