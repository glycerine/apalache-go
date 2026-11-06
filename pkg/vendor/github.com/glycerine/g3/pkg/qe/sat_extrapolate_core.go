package qe

// Source: src/qe/qe_sat_tactic.cpp, smt_test_extrapolate.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// satCoreExtrapolate returns an owned conjunction of assignment literals
// selected by the solver's unsatisfiable assumption core.
func satCoreExtrapolate(m *ast.ASTManager, solver *smtcore.SMTSolver, b ast.ASTHandle, assignments []ast.ASTHandle, checkpoint func() error) (result ast.ASTRef, err error) {
	if m == nil || solver == nil {
		return result, ast.ErrASTHandle
	}
	if checkpoint != nil {
		if err = checkpoint(); err != nil {
			return result, err
		}
	}
	if err = solver.Push(); err != nil {
		return result, err
	}
	defer func() {
		err = errors.Join(err, solver.Pop(1))
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	notB, err := m.MkNot(b)
	if err != nil {
		return result, err
	}
	if err = solver.AssertFormula(notB); err != nil {
		return result, err
	}
	sort, err := m.MkBoolSort()
	if err != nil {
		return result, err
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
	}()
	proxies := make([]ast.ASTHandle, 0, len(assignments))
	mapping := make(map[ast.ASTHandle]ast.ASTHandle, len(assignments))
	for _, assignment := range assignments {
		proxy, e := m.MkFreshConst("proxy", sort)
		if e != nil {
			return result, e
		}
		ref, e := m.NewRef(proxy)
		if e != nil {
			return result, e
		}
		refs = append(refs, ref)
		proxies = append(proxies, proxy)
		mapping[proxy] = assignment
		equal, e := m.MkIff(proxy, assignment)
		if e != nil {
			return result, e
		}
		if e = solver.AssertFormula(equal); e != nil {
			return result, e
		}
	}
	status, err := solver.CheckSat(proxies)
	if err != nil {
		return result, err
	}
	if status != lbool.False {
		return result, tactic.NewTacticException("qe-sat extrapolation did not produce an unsatisfiable core")
	}
	core, err := solver.UnsatCore()
	if err != nil {
		return result, err
	}
	literals := make([]ast.ASTHandle, 0, len(core))
	for _, proxy := range core {
		lit, ok := mapping[proxy]
		if !ok {
			return result, ast.ErrASTDomain
		}
		literals = append(literals, lit)
	}
	conjunction, err := m.MkAndArgs(literals)
	if err != nil {
		return result, err
	}
	return m.NewRef(conjunction)
}
