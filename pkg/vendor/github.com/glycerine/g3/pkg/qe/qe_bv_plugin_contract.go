package qe

// Source: src/qe/qe_bv_plugin.cpp retained callbacks not represented by the
// minimal SolverPlugin interface. Keeping the concrete contract explicit lets
// tactic hosts use branching without an any-valued adapter.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type BitVectorBranchPlugin interface {
	SolverPlugin
	GetNumBranches(*ast.ContainsApp, ast.ASTHandle, *numeral.OwnedRational) (bool, error)
	Assign(*ast.ContainsApp, ast.ASTHandle, *numeral.OwnedRational) error
	Substitute(*ast.ContainsApp, *numeral.OwnedRational, ast.ASTRef, *ast.ASTRef) error
	Weight(*ast.ContainsApp, ast.ASTHandle) (uint32, error)
	Solve(*ConjunctionEnum, ast.ASTHandle) (bool, error)
}

var _ BitVectorBranchPlugin = (*BitVectorPlugin)(nil)
