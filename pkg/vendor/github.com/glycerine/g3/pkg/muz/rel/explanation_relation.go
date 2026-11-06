package rel

// Source: src/muz/rel/dl_mk_explanations.cpp, explanation_relation storage,
// reset, fact insertion, clone and complement. Relation operations and the
// explanation rule transformer are separate port steps.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ExplanationRelation stores at most one row of rule-sort expressions. A
// nonempty row may contain null slots representing undefined explanations.
// The relation owns its signature and row AST references.
type ExplanationRelation struct {
	manager *ast.ASTManager
	sorts   []ast.ASTRef
	data    []ast.ASTRef
	empty   bool
	closed  bool
}

func NewExplanationRelation(manager *ast.ASTManager, sorts []ast.ASTHandle) (_ *ExplanationRelation, resultErr error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	r := &ExplanationRelation{manager: manager, empty: true}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, r.Close())
		}
	}()
	for _, sort := range sorts {
		ok, err := manager.IsDatalogRuleSort(sort)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ast.ErrASTDomain
		}
		ref, err := manager.NewRef(sort)
		if err != nil {
			return nil, err
		}
		r.sorts = append(r.sorts, ref)
	}
	return r, nil
}

func (r *ExplanationRelation) open() error {
	if r == nil || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (r *ExplanationRelation) Empty() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return r.empty, nil
}

func (r *ExplanationRelation) SizeEstimate() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	if r.empty {
		return 0, nil
	}
	return 1, nil
}

func (r *ExplanationRelation) NoUndefined() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	if r.empty {
		return true, nil
	}
	for _, ref := range r.data {
		if ref == (ast.ASTRef{}) {
			return false, nil
		}
	}
	return true, nil
}

// Data returns borrowed row expressions; null handles denote undefined slots.
func (r *ExplanationRelation) Data() ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.empty {
		return nil, nil
	}
	data := make([]ast.ASTHandle, len(r.data))
	for i, ref := range r.data {
		if ref == (ast.ASTRef{}) {
			continue
		}
		var err error
		data[i], err = r.manager.RefGet(ref)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func (r *ExplanationRelation) assignData(data []ast.ASTHandle) (resultErr error) {
	if len(data) != len(r.sorts) {
		return ast.ErrASTDomain
	}
	refs := make([]ast.ASTRef, len(data))
	defer func() {
		if resultErr != nil {
			for i := len(refs) - 1; i >= 0; i-- {
				if refs[i] != (ast.ASTRef{}) {
					resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
				}
			}
		}
	}()
	for i, value := range data {
		if value.IsNull() {
			continue
		}
		kind, err := r.manager.GetKind(value)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			return ast.ErrASTDomain
		}
		sort, err := r.manager.GetSort(value)
		if err != nil {
			return err
		}
		want, err := r.manager.RefGet(r.sorts[i])
		if err != nil {
			return err
		}
		if sort != want {
			return ast.ErrASTDomain
		}
		refs[i], err = r.manager.NewRef(value)
		if err != nil {
			return err
		}
	}
	for i := len(r.data) - 1; i >= 0; i-- {
		if r.data[i] != (ast.ASTRef{}) {
			if err := r.manager.DelRef(&r.data[i]); err != nil {
				return err
			}
		}
	}
	r.data, r.empty = refs, false
	return nil
}

// AddFact follows the source single-fact precondition.
func (r *ExplanationRelation) AddFact(data []ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	if !r.empty {
		return ast.ErrASTDomain
	}
	return r.assignData(data)
}

// Reset leaves the old row owned until replacement or Close, as the source does.
func (r *ExplanationRelation) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	r.empty = true
	return nil
}

func (r *ExplanationRelation) signature() ([]ast.ASTHandle, error) {
	sorts := make([]ast.ASTHandle, len(r.sorts))
	for i, ref := range r.sorts {
		value, err := r.manager.RefGet(ref)
		if err != nil {
			return nil, err
		}
		sorts[i] = value
	}
	return sorts, nil
}

func (r *ExplanationRelation) Clone() (_ *ExplanationRelation, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	sorts, err := r.signature()
	if err != nil {
		return nil, err
	}
	clone, err := NewExplanationRelation(r.manager, sorts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, clone.Close())
		}
	}()
	if len(r.data) != 0 {
		values := make([]ast.ASTHandle, len(r.data))
		for i, ref := range r.data {
			if ref == (ast.ASTRef{}) {
				continue
			}
			values[i], err = r.manager.RefGet(ref)
			if err != nil {
				return nil, err
			}
		}
		if err = clone.assignData(values); err != nil {
			return nil, err
		}
	}
	clone.empty = r.empty
	return clone, nil
}

func (r *ExplanationRelation) Complement() (_ *ExplanationRelation, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	sorts, err := r.signature()
	if err != nil {
		return nil, err
	}
	result, err := NewExplanationRelation(r.manager, sorts)
	if err != nil {
		return nil, err
	}
	if r.empty {
		if err = result.assignData(make([]ast.ASTHandle, len(sorts))); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

func (r *ExplanationRelation) Close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	for i := len(r.data) - 1; i >= 0; i-- {
		if r.data[i] != (ast.ASTRef{}) {
			result = errors.Join(result, r.manager.DelRef(&r.data[i]))
		}
	}
	for i := len(r.sorts) - 1; i >= 0; i-- {
		if r.sorts[i] != (ast.ASTRef{}) {
			result = errors.Join(result, r.manager.DelRef(&r.sorts[i]))
		}
	}
	if result == nil {
		r.data, r.sorts, r.manager, r.closed = nil, nil, nil, true
	}
	return result
}
