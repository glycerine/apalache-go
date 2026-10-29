package core

// Source: src/tactic/core/symmetry_reduce_tactic.cpp ac_rewriter and cfg.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type symmetryRewriteConfig struct {
	rewriter.DefaultConfiguration
	manager *ast.ASTManager
}

func (c *symmetryRewriteConfig) RewritePatterns() bool                 { return false }
func (c *symmetryRewriteConfig) FlatAssoc(ast.ASTHandle) (bool, error) { return false, nil }
func (c *symmetryRewriteConfig) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	m := c.manager
	flags, err := m.GetFuncDeclFlags(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	family, err := m.GetFamilyID(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	all := flags.LeftAssociative && flags.RightAssociative && flags.Commutative || family == m.BasicFamilyID() && op == ast.ASTOpDistinct
	if !all && !(flags.Commutative && args.Count() == 2) {
		return rewriter.RewriteFailed, nil
	}
	type argument struct {
		term ast.ASTHandle
		id   uint32
	}
	ordered := make([]argument, args.Count())
	for i := range ordered {
		h, e := args.At(uint32(i))
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		id, e := m.GetASTID(h)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		ordered[i] = argument{h, id}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].id < ordered[j].id })
	changed := false
	values := make([]ast.ASTHandle, len(ordered))
	for i, a := range ordered {
		values[i] = a.term
		old, e := args.At(uint32(i))
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		changed = changed || old != a.term
	}
	if !changed {
		return rewriter.RewriteFailed, nil
	}
	h, err := m.MkApp(d, values)
	if err == nil {
		err = m.SetRef(out, h)
	}
	if err == nil {
		err = m.SetRef(proof, ast.ASTHandle{})
	}
	return rewriter.RewriteDone, err
}

func (t *SymmetryReduceTactic) normalize(term ast.ASTHandle, saved *solveEquationFrame) (value ast.ASTHandle, err error) {
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return value, err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	if err = t.rewriter.Rewrite(term, out); err != nil {
		return value, err
	}
	value, err = t.manager.RefGet(out)
	if err == nil {
		err = saved.pin(value)
	}
	return value, err
}

func (t *SymmetryReduceTactic) checkPermutation(term ast.ASTHandle, constants []ast.ASTHandle, cycle bool, saved *solveEquationFrame) (ok bool, err error) {
	m := t.manager
	sub, err := m.NewExprSubstitution()
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&sub)) }()
	count := len(constants)
	if !cycle {
		count = 2
	}
	for i := 0; i < count; i++ {
		if err = m.ExprSubstitutionInsert(sub, constants[i], constants[(i+1)%count], ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return false, err
		}
	}
	if err = t.replacer.SetSubstitution(&sub); err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, t.replacer.SetSubstitution(nil)) }()
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	if err = t.replacer.Rewrite(term, out); err != nil {
		return false, err
	}
	h, err := m.RefGet(out)
	if err != nil {
		return false, err
	}
	h, err = t.normalize(h, saved)
	return term == h, err
}
