package tab

// Source: src/muz/tab/tab_context.cpp, unifier::unify QE and variable compaction.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
)

func finalizeUnfoldedClause(u *unfoldedClause, rw *rewriter.TheoryRewriter, lite *qe.Lite) (result *Clause, matched bool, err error) {
	if u == nil || u.manager == nil || rw == nil || rw.Manager() != u.manager || lite == nil {
		return nil, false, ast.ErrASTHandle
	}
	m := u.manager
	attempt := &unfoldFinalizeAttempt{manager: m}
	defer func() { result, matched, err = finishUnfoldFinalize(attempt, matched, err) }()
	head, err := m.RefGet(u.head)
	if err != nil {
		return nil, false, err
	}
	predicates := make([]ast.ASTHandle, len(u.predicates))
	for i, ref := range u.predicates {
		predicates[i], err = m.RefGet(ref)
		if err != nil {
			return nil, false, err
		}
	}
	free, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return nil, false, err
	}
	if err = free.Accumulate(head); err != nil {
		return nil, false, err
	}
	for _, p := range predicates {
		if err = free.Accumulate(p); err != nil {
			return nil, false, err
		}
	}
	var keep []uint32
	for i := uint32(0); i < free.Size(); i++ {
		if free.Contains(i) {
			keep = append(keep, i)
		}
	}
	constraint, err := m.CopyRef(u.constraint)
	if err != nil {
		return nil, false, err
	}
	attempt.temps = append(attempt.temps, constraint)
	if err = lite.EliminateIndices(keep, false, constraint); err != nil {
		return nil, false, err
	}
	condition, err := m.RefGet(constraint)
	if err != nil {
		return nil, false, err
	}
	if m.IsFalse(condition) {
		return nil, false, nil
	}
	work, err := NewClause(m, rw, head, predicates, condition)
	if err != nil {
		return nil, false, err
	}
	attempt.work = work
	free, err = work.freeVariables()
	if err != nil {
		return nil, false, err
	}
	renaming := make([]ast.ASTHandle, free.Size())
	if u.bindings != nil {
		for i := range u.rename {
			if u.rename[i] != (ast.ASTRef{}) {
				if err = m.DelRef(&u.rename[i]); err != nil {
					return nil, false, err
				}
			}
		}
		u.rename = make([]ast.ASTRef, free.Size())
	}
	next := uint32(0)
	changed := false
	for i := range renaming {
		sort, e := free.Get(uint32(i))
		if e != nil {
			return nil, false, e
		}
		if sort.IsNull() {
			changed = true
			continue
		}
		renaming[i], e = m.MkVar(next, sort)
		if e != nil {
			return nil, false, e
		}
		ref, e := m.NewRef(renaming[i])
		if e != nil {
			return nil, false, e
		}
		if u.bindings != nil {
			u.rename[i] = ref
		} else {
			attempt.temps = append(attempt.temps, ref)
		}
		next++
	}
	if !changed {
		result, attempt.work = work, nil
		attempt.result = result
		return result, true, nil
	}
	subst, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return nil, false, err
	}
	attempt.subst = subst
	apply := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTHandle{}, e
		}
		attempt.temps = append(attempt.temps, ref)
		if e = subst.Substitute(h, renaming, ref); e != nil {
			return ast.ASTHandle{}, e
		}
		return m.RefGet(ref)
	}
	head, err = work.Head()
	if err != nil {
		return nil, false, err
	}
	head, err = apply(head)
	if err != nil {
		return nil, false, err
	}
	predicates, err = work.Predicates()
	if err != nil {
		return nil, false, err
	}
	for i, p := range predicates {
		predicates[i], err = apply(p)
		if err != nil {
			return nil, false, err
		}
	}
	condition, err = work.Constraint()
	if err != nil {
		return nil, false, err
	}
	condition, err = apply(condition)
	if err != nil {
		return nil, false, err
	}
	result, err = NewClause(m, rw, head, predicates, condition)
	if err != nil {
		return nil, false, err
	}
	attempt.result = result
	return result, true, nil
}
