package rel

// Source: src/muz/rel/dl_mk_explanations.cpp, assignment, negation and
// intersection filters for explanation relations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpreted recognizes a rule-sort variable equated to an application
// and assigns the corresponding undefined column. Unsupported shapes return
// false without changing the relation.
func (r *ExplanationRelation) FilterInterpreted(condition ast.ASTHandle) (matched bool, resultErr error) {
	if err := r.open(); err != nil {
		return false, err
	}
	m := r.manager
	family, op, left, right, binary, err := intervalParts(m, condition)
	if err != nil || !binary {
		return false, err
	}
	if family != m.BasicFamilyID() || op != ast.ASTOpEq {
		return false, nil
	}
	rightKind, err := m.GetKind(right)
	if err != nil {
		return false, err
	}
	if rightKind == ast.ASTVar {
		left, right = right, left
	}
	leftKind, err := m.GetKind(left)
	if err != nil {
		return false, err
	}
	rightKind, err = m.GetKind(right)
	if err != nil {
		return false, err
	}
	if leftKind != ast.ASTVar || rightKind != ast.ASTApp {
		return false, nil
	}
	sort, err := m.GetSort(left)
	if err != nil {
		return false, err
	}
	isRule, err := m.IsDatalogRuleSort(sort)
	if err != nil || !isRule {
		return false, err
	}
	index, err := m.GetVarIndex(left)
	if err != nil {
		return false, err
	}
	if r.empty || uint64(index) >= uint64(len(r.data)) || r.data[index] != (ast.ASTRef{}) {
		return false, ast.ErrASTDomain
	}
	bindings := make([]ast.ASTHandle, len(r.data))
	for i, ref := range r.data {
		if ref != (ast.ASTRef{}) {
			bindings[len(bindings)-1-i], err = m.RefGet(ref)
			if err != nil {
				return false, err
			}
		}
	}
	// dl_context constructs its shared var_subst in standard order.
	sub, err := rewriter.NewVarSubstitution(m, true)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, m.DelRef(&ref))
		}
	}()
	if err = sub.Substitute(right, bindings, ref); err != nil {
		return false, err
	}
	value, err := m.RefGet(ref)
	if err != nil {
		return false, err
	}
	kind, err := m.GetKind(value)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	valueSort, err := m.GetSort(value)
	if err != nil {
		return false, err
	}
	want, err := m.RefGet(r.sorts[index])
	if err != nil {
		return false, err
	}
	if valueSort != want {
		return false, ast.ErrASTDomain
	}
	r.data[index], keep = ref, true
	return true, nil
}

// FilterNegation drops the target whenever the negated explanation is nonempty.
func (r *ExplanationRelation) FilterNegation(negative *ExplanationRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if err := negative.open(); err != nil {
		return err
	}
	if r.manager != negative.manager {
		return ast.ErrASTHandle
	}
	if !negative.empty {
		return r.Reset()
	}
	return nil
}

// FilterIntersection implements the source full-column one-to-one
// intersection, including its intentionally imprecise explanation narrowing.
func (r *ExplanationRelation) FilterIntersection(source *ExplanationRelation) (resultErr error) {
	if err := r.compatible(source); err != nil {
		return err
	}
	if source.empty {
		return r.Reset()
	}
	if r.empty {
		return nil
	}
	target, err := r.Data()
	if err != nil {
		return err
	}
	incoming, err := source.Data()
	if err != nil {
		return err
	}
	name, err := r.manager.Symbols.Intern("e_union")
	if err != nil {
		return err
	}
	for i := range target {
		if incoming[i].IsNull() {
			continue
		}
		if target[i].IsNull() {
			target[i] = incoming[i]
			continue
		}
		sort, err := r.manager.RefGet(r.sorts[i])
		if err != nil {
			return err
		}
		union, err := r.manager.MkFuncDecl(name, []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
		if err != nil {
			return err
		}
		decl, err := r.manager.GetDecl(target[i])
		if err != nil {
			return err
		}
		if decl != union {
			continue
		}
		left, err := r.manager.GetArg(target[i], 0)
		if err != nil {
			return err
		}
		right, err := r.manager.GetArg(target[i], 1)
		if err != nil {
			return err
		}
		if left == incoming[i] || right == incoming[i] {
			target[i] = incoming[i]
		}
	}
	return r.assignData(target)
}
