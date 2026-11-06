package tab

// Source: src/muz/tab/tab_context.cpp, unifier::unify substitution phase.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
)

type unfoldedClause struct {
	manager          *ast.ASTManager
	head, constraint ast.ASTRef
	predicates       []ast.ASTRef
	bindings         *substitution.Substitution
	rename           []ast.ASTRef
	maxVars          uint32
}

func (u *unfoldedClause) Close() error {
	if u == nil || u.manager == nil {
		return nil
	}
	if u.bindings != nil {
		if err := u.bindings.Close(); err != nil {
			return err
		}
		u.bindings = nil
	}
	for i := range u.rename {
		if u.rename[i] != (ast.ASTRef{}) {
			if err := u.manager.DelRef(&u.rename[i]); err != nil {
				return err
			}
		}
	}
	u.rename = nil
	for i := range u.predicates {
		if u.predicates[i] != (ast.ASTRef{}) {
			if err := u.manager.DelRef(&u.predicates[i]); err != nil {
				return err
			}
		}
	}
	for _, ref := range []*ast.ASTRef{&u.head, &u.constraint} {
		if *ref != (ast.ASTRef{}) {
			if err := u.manager.DelRef(ref); err != nil {
				return err
			}
		}
	}
	u.predicates = nil
	u.manager = nil
	return nil
}

// unfoldClauseExpressions unifies two source variable banks and publishes the
// substituted head, ordered predicates, and conjunction before QE.
func unfoldClauseExpressions(target *Clause, selected uint32, source *Clause) (result *unfoldedClause, matched bool, err error) {
	return unfoldClauseExpressionsWithBindings(target, selected, source, false)
}

func unfoldClauseExpressionsWithBindings(target *Clause, selected uint32, source *Clause, saveBindings bool) (result *unfoldedClause, matched bool, err error) {
	if target == nil || source == nil || target.closed || source.closed || target.manager != source.manager {
		return nil, false, ast.ErrASTHandle
	}
	m := target.manager
	attempt := &unfoldConstructionAttempt{manager: m}
	defer func() { result, matched, err = finishUnfoldConstruction(attempt, matched, err) }()
	targetPreds, err := target.Predicates()
	if err != nil {
		return nil, false, err
	}
	if uint64(selected) >= uint64(len(targetPreds)) {
		return nil, false, ast.ErrASTDomain
	}
	sourceHead, err := source.Head()
	if err != nil {
		return nil, false, err
	}
	subst, err := substitution.New(m)
	if err != nil {
		return nil, false, err
	}
	attempt.substitution = subst
	count := target.numVars
	if source.numVars > count {
		count = source.numVars
	}
	if err = subst.Reserve(2, count); err != nil {
		return nil, false, err
	}
	unifier, err := substitution.NewUnifier(m)
	if err != nil {
		return nil, false, err
	}
	attempt.unifier = unifier
	matched, err = unifier.UnifyPair(targetPreds[selected], sourceHead, subst, true)
	if err != nil || !matched {
		return nil, matched, err
	}
	result = &unfoldedClause{manager: m}
	attempt.result = result
	apply := func(h ast.ASTHandle, offset uint32) (ref ast.ASTRef, e error) {
		ref, e = m.NewRef(ast.ASTHandle{})
		if e != nil {
			return ref, e
		}
		if e = subst.Apply(2, []uint32{0, count}, substitution.NewExprOffset(h, offset), ref); e != nil {
			attempt.temporary = append(attempt.temporary, ref)
			ref = ast.ASTRef{}
		}
		return ref, e
	}
	head, err := target.Head()
	if err != nil {
		return result, false, err
	}
	result.head, err = apply(head, 0)
	if err != nil {
		return result, false, err
	}
	sourcePreds, err := source.Predicates()
	if err != nil {
		return result, false, err
	}
	for i, pred := range targetPreds {
		if uint32(i) == selected {
			for _, fromRule := range sourcePreds {
				ref, e := apply(fromRule, 1)
				if e != nil {
					return result, false, e
				}
				result.predicates = append(result.predicates, ref)
			}
		} else {
			ref, e := apply(pred, 0)
			if e != nil {
				return result, false, e
			}
			result.predicates = append(result.predicates, ref)
		}
	}
	targetConstraint, err := target.Constraint()
	if err != nil {
		return result, false, err
	}
	sourceConstraint, err := source.Constraint()
	if err != nil {
		return result, false, err
	}
	left, err := apply(targetConstraint, 0)
	if err != nil {
		return result, false, err
	}
	attempt.left = left
	right, err := apply(sourceConstraint, 1)
	if err != nil {
		return result, false, err
	}
	attempt.right = right
	a, err := m.RefGet(left)
	if err != nil {
		return result, false, err
	}
	b, err := m.RefGet(right)
	if err != nil {
		return result, false, err
	}
	conjunction, err := m.MkAnd(a, b)
	if err != nil {
		return result, false, err
	}
	result.constraint, err = m.NewRef(conjunction)
	if err != nil {
		return result, false, err
	}
	if saveBindings {
		result.bindings = subst
		result.maxVars = count
		attempt.substitution = nil
	}
	return result, true, nil
}
