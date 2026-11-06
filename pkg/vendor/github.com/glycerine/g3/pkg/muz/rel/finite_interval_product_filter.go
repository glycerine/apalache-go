package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp, the inner-only path of
// filter_interpreted_fn.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpretedInner applies a condition that refers only to interval
// columns. Conditions mentioning finite table columns require the source's
// separate table or mixed filtering paths, which are not implemented here.
func (p *FiniteIntervalProduct) FilterInterpretedInner(condition ast.ASTHandle) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	m := p.manager.manager
	kind, err := m.GetKind(condition)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	sort, err := m.GetSort(condition)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if sort != boolean {
		return ast.ErrASTDomain
	}
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil {
		return err
	}
	if uint64(used.MaxFoundVarIndexPlusOne()) > uint64(len(p.sorts)) {
		return ast.ErrASTDomain
	}
	for _, column := range p.tableColumns {
		if used.Contains(uint32(column)) != (ast.ASTHandle{}) {
			return ErrTableRelation
		}
	}
	// Ground conditions use the source table filter rather than this path.
	if used.NumVars() == 0 {
		return ErrTableRelation
	}
	bindings := make([]ast.ASTHandle, len(p.sorts))
	refs := make([]ast.ASTRef, 0, len(p.intervalColumns)+1)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	for local, global := range p.intervalColumns {
		variable, e := m.MkVar(uint32(local), p.sorts[global])
		if e != nil {
			return e
		}
		ref, e := m.NewRef(variable)
		if e != nil {
			return e
		}
		refs = append(refs, ref)
		bindings[global] = variable
	}
	subst, err := rewriter.NewVarSubstitution(m, false)
	if subst != nil {
		defer func() { resultErr = errors.Join(resultErr, subst.Close()) }()
	}
	if err != nil {
		return err
	}
	rewritten, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	refs = append(refs, rewritten)
	if err = subst.Substitute(condition, bindings, rewritten); err != nil {
		return err
	}
	filtered, err := m.RefGet(rewritten)
	if err != nil {
		return err
	}
	if err = p.garbageCollect(false); err != nil {
		return err
	}
	for _, inner := range p.intervals {
		if inner != nil {
			if err = inner.FilterInterpreted(m, filtered); err != nil {
				return err
			}
		}
	}
	return nil
}
