package rel

// Source: src/muz/rel/dl_mk_explanations.cpp, explanation relation join,
// projection, rename and union functors.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Join concatenates two explanation rows. Source joins with equality columns
// are not supported by the explanation plugin.
func (r *ExplanationRelation) Join(other *ExplanationRelation) (_ *ExplanationRelation, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if err := other.open(); err != nil {
		return nil, err
	}
	if r.manager != other.manager {
		return nil, ast.ErrASTHandle
	}
	left, err := r.signature()
	if err != nil {
		return nil, err
	}
	right, err := other.signature()
	if err != nil {
		return nil, err
	}
	result, err := NewExplanationRelation(r.manager, append(left, right...))
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if !r.empty && !other.empty {
		a, err := r.Data()
		if err != nil {
			return nil, err
		}
		b, err := other.Data()
		if err != nil {
			return nil, err
		}
		if err = result.assignData(append(a, b...)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *ExplanationRelation) Project(removed []int) (_ *ExplanationRelation, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	flags, err := tableRelationRemovedColumns(len(r.sorts), removed)
	if err != nil {
		return nil, err
	}
	sorts, err := r.signature()
	if err != nil {
		return nil, err
	}
	var keepSorts []ast.ASTHandle
	for i, sort := range sorts {
		if !flags[i] {
			keepSorts = append(keepSorts, sort)
		}
	}
	result, err := NewExplanationRelation(r.manager, keepSorts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if !r.empty {
		data, err := r.Data()
		if err != nil {
			return nil, err
		}
		var keep []ast.ASTHandle
		for i, value := range data {
			if !flags[i] {
				keep = append(keep, value)
			}
		}
		if err = result.assignData(keep); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *ExplanationRelation) RenameCycle(cycle []int) (_ *ExplanationRelation, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	permutation, err := tableRelationCyclePermutation(len(r.sorts), cycle)
	if err != nil {
		return nil, err
	}
	sorts, err := r.signature()
	if err != nil {
		return nil, err
	}
	renamed := make([]ast.ASTHandle, len(sorts))
	for i, from := range permutation {
		renamed[i] = sorts[from]
	}
	result, err := NewExplanationRelation(r.manager, renamed)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if !r.empty {
		data, err := r.Data()
		if err != nil {
			return nil, err
		}
		row := make([]ast.ASTHandle, len(data))
		for i, from := range permutation {
			row[i] = data[from]
		}
		if err = result.assignData(row); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *ExplanationRelation) compatible(other *ExplanationRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if r.manager != other.manager || len(r.sorts) != len(other.sorts) {
		return ast.ErrASTDomain
	}
	for i := range r.sorts {
		left, err := r.manager.RefGet(r.sorts[i])
		if err != nil {
			return err
		}
		right, err := r.manager.RefGet(other.sorts[i])
		if err != nil {
			return err
		}
		if left != right {
			return ast.ErrASTDomain
		}
	}
	return nil
}

// Union follows either the source fact-level first-explanation policy or the
// relation-level e_union aggregation. Delta may be nil.
func (r *ExplanationRelation) Union(source, delta *ExplanationRelation, relationLevel bool) error {
	if err := r.compatible(source); err != nil {
		return err
	}
	if delta != nil {
		if err := r.compatible(delta); err != nil {
			return err
		}
	}
	for _, relation := range []*ExplanationRelation{r, source, delta} {
		if relation == nil {
			continue
		}
		defined, err := relation.NoUndefined()
		if err != nil {
			return err
		}
		if !defined {
			return ast.ErrASTDomain
		}
	}
	if source.empty {
		return nil
	}
	if relationLevel {
		if err := r.unite(source); err != nil {
			return err
		}
		if delta != nil {
			return delta.unite(source)
		}
		return nil
	}
	if r.empty {
		data, err := source.Data()
		if err != nil {
			return err
		}
		if err = r.assignData(data); err != nil {
			return err
		}
		if delta != nil && delta.empty {
			return delta.assignData(data)
		}
	}
	return nil
}

func (r *ExplanationRelation) unite(source *ExplanationRelation) (resultErr error) {
	data, err := source.Data()
	if err != nil {
		return err
	}
	if r.empty {
		return r.assignData(data)
	}
	current, err := r.Data()
	if err != nil {
		return err
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
	}()
	name, err := r.manager.Symbols.Intern("e_union")
	if err != nil {
		return err
	}
	for i := range current {
		sort, err := r.manager.RefGet(r.sorts[i])
		if err != nil {
			return err
		}
		decl, err := r.manager.MkFuncDecl(name, []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
		if err != nil {
			return err
		}
		term, err := r.manager.MkApp(decl, []ast.ASTHandle{current[i], data[i]})
		if err != nil {
			return err
		}
		ref, err := r.manager.NewRef(term)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		current[i] = term
	}
	return r.assignData(current)
}
