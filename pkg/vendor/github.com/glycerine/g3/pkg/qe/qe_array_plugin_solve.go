package qe

// Source: src/qe/qe_array_plugin.cpp, explicit-equality part of solve.

import "github.com/glycerine/g3/pkg/ast"

// solveArraySide uses source precedence: direct variable, store, then select.
func (p *ArrayPlugin) solveArraySide(lhs, rhs, formula ast.ASTHandle) (bool, error) {
	solved, e := p.tryDirectSide(lhs, rhs, formula)
	if e != nil || solved {
		return solved, e
	}
	solved, e = p.TryStoreEquality(lhs, rhs, formula)
	if e != nil || solved {
		return solved, e
	}
	return p.TrySelectEquality(lhs, rhs, formula)
}

// SolveExplicitEqualities scans conjunctions in source order and stops after
// the first successful direct, store or select equality elimination.
// The source's subsequent arithmetic-zero pass is not included yet.
func (p *ArrayPlugin) SolveExplicitEqualities(conjs *ConjunctionEnum, formula ast.ASTHandle) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	if conjs == nil || conjs.manager != p.manager {
		return false, ast.ErrASTHandle
	}
	n, e := conjs.Len()
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		eq, e := conjs.At(i)
		if e != nil {
			return false, e
		}
		left, right, ok, e := p.equalitySides(eq)
		if e != nil {
			return false, e
		}
		if !ok {
			continue
		}
		solved, e := p.solveArraySide(left, right, formula)
		if e != nil || solved {
			return solved, e
		}
		solved, e = p.solveArraySide(right, left, formula)
		if e != nil || solved {
			return solved, e
		}
	}
	return false, nil
}
