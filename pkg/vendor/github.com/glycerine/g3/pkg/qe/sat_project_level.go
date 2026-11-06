package qe

// Source: src/qe/qe_sat_tactic.cpp, project with partial projection mode.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func projectSATPartialLevel(levels *satLevels, index uint32, variables []ast.ASTHandle, formula ast.ASTHandle, config Config, rw *rewriter.TheoryRewriter, strong *smtcore.ExprStrongContextSimplifier, local, full bool) (err error) {
	if levels == nil || levels.manager == nil || rw == nil {
		return ast.ErrASTHandle
	}
	if index+1 >= uint32(len(levels.levels)) {
		return arena.ErrBounds
	}
	m := levels.manager
	previous, err := m.RefGet(levels.levels[index].formula)
	if err != nil {
		return err
	}
	if strong != nil && local {
		ref, e := simplifySATContext(m, strong, previous, formula)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, m.DelRef(&ref)) }()
		formula, err = m.RefGet(ref)
		if err != nil {
			return err
		}
	}
	next := levels.levels[index+1].solver
	context, err := newSATSolverContext(m, next, config)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, context.Close()) }()
	for _, v := range variables {
		if err = context.addVariable(v); err != nil {
			return err
		}
	}
	if err = context.initializeFormula(formula); err != nil {
		return err
	}
	if err = next.Push(); err != nil {
		return err
	}
	scoped := true
	defer func() {
		if scoped {
			err = errors.Join(err, next.Pop(1))
		}
	}()
	for len(context.variables) > 0 {
		status, e := next.CheckSat(nil)
		if e != nil {
			return e
		}
		if status != lbool.True {
			return tactic.NewTacticException("qe-sat projection requires a satisfiable successor")
		}
		if full {
			if e = context.projectFull(uint32(len(context.variables)-1), rw, config); e != nil {
				return e
			}
			continue
		}
		md, e := next.AcquireModel()
		if e != nil {
			return e
		}
		if md == nil {
			return ast.ErrASTDomain
		}
		derived := md.DerivedModel()
		if derived == nil {
			return errors.Join(ast.ErrASTDomain, md.DecRef())
		}
		e = context.projectPartial(uint32(len(context.variables)-1), derived, rw, config)
		e = errors.Join(e, md.DecRef())
		if e != nil {
			return e
		}
	}
	if err = next.Pop(1); err != nil {
		return err
	}
	scoped = false
	projected, err := m.RefGet(context.formula)
	if err != nil {
		return err
	}
	negated, err := m.MkNot(projected)
	if err != nil {
		return err
	}
	negRef, err := m.NewRef(negated)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&negRef)) }()
	if err = rw.Rewrite(negated, negRef); err != nil {
		return err
	}
	negated, err = m.RefGet(negRef)
	if err != nil {
		return err
	}
	if strong != nil && !local {
		ref, e := simplifySATContext(m, strong, previous, negated)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, m.DelRef(&ref)) }()
		if err = m.AssignRef(negRef, ref); err != nil {
			return err
		}
		negated, err = m.RefGet(negRef)
		if err != nil {
			return err
		}
	}
	conjunction, err := m.MkAnd(previous, negated)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(conjunction)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&ref)) }()
	if err = rw.Rewrite(conjunction, ref); err != nil {
		return err
	}
	if err = m.AssignRef(levels.levels[index].formula, ref); err != nil {
		return err
	}
	return levels.levels[index].solver.AssertFormula(negated)
}

// simplifySATContext limits each source-level simplification to the current M(i).
func simplifySATContext(m *ast.ASTManager, strong *smtcore.ExprStrongContextSimplifier, context, formula ast.ASTHandle) (result ast.ASTRef, err error) {
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	if err = strong.Push(); err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, strong.Pop()) }()
	if err = strong.Assert(context); err != nil {
		return result, err
	}
	return strong.Simplify(formula)
}
