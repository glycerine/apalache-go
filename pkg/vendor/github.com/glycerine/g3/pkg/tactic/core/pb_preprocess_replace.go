package core

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *PBPreprocessTactic) replace(g *tactic.Goal, positions []uint32, variable, value ast.ASTHandle) (_ bool, result error) {
	m := t.manager
	sub, e := m.NewExprSubstitution()
	if e != nil {
		return false, e
	}
	defer func() { result = errors.Join(result, m.DelExprSubstitution(&sub)) }()
	if e = m.ExprSubstitutionInsert(sub, variable, value, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
		return false, e
	}
	r, e := rewriter.NewTheoryRewriter(m, t.services.Context, t.services.Globals, nil)
	if e != nil {
		return false, e
	}
	defer func() { result = errors.Join(result, r.Close()) }()
	if e = r.SetSubstitution(&sub); e != nil {
		return false, e
	}
	out, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return false, e
	}
	defer func() { result = errors.Join(result, m.DelRef(&out)) }()
	progress := false
	for _, index := range positions {
		h, x := g.Form(index)
		if x != nil {
			return false, x
		}
		if m.IsTrue(h) {
			continue
		}
		if x = r.Rewrite(h, out); x != nil {
			return false, x
		}
		next, x := m.RefGet(out)
		if x != nil {
			return false, x
		}
		if next == h {
			continue
		}
		pr, x := g.Proof(index)
		if x != nil {
			return false, x
		}
		dep, x := g.Dependency(index)
		if x != nil {
			return false, x
		}
		if x = g.Update(index, next, pr, dep); x != nil {
			return false, x
		}
		progress = true
	}
	return progress, nil
}
