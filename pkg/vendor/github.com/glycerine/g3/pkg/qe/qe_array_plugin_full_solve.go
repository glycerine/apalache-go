package qe

// Source: src/qe/qe_array_plugin.cpp, solve and reached solve_eq_zero guard.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// Solve runs the source explicit-equality pass and its arithmetic extraction
// fallback. ExtractEqualities constructs OP_SUB nodes; the source fallback
// only processes OP_ADD nodes, so its rewriting body is unreachable here.
func (p *ArrayPlugin) Solve(conjs *ConjunctionEnum, formula ast.ASTHandle) (solved bool, err error) {
	solved, err = p.SolveExplicitEqualities(conjs, formula)
	if err != nil || solved {
		return solved, err
	}
	equalities, err := conjs.ExtractEqualities()
	if err != nil {
		return false, err
	}
	defer func() {
		for i := len(equalities) - 1; i >= 0; i-- {
			err = errors.Join(err, p.manager.DelRef(&equalities[i]))
		}
	}()
	if len(equalities) == 0 {
		return false, nil
	}
	family, err := p.manager.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	for _, ref := range equalities {
		term, e := p.manager.RefGet(ref)
		if e != nil {
			return false, e
		}
		fid, e := p.manager.GetFamilyID(term)
		if e != nil {
			return false, e
		}
		op, e := p.manager.GetDeclKind(term)
		if e != nil {
			return false, e
		}
		// Guard the extraction invariant instead of silently accepting an OP_ADD
		// if that API is changed independently in future.
		if fid == family && op == 6 {
			return false, ast.ErrASTDomain
		}
	}
	return false, nil
}
