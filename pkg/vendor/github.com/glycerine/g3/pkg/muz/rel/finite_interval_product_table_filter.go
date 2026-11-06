package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp, table-only interpreted
// filter, and dl_relation_manager.cpp's generic interpreted table filter.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpretedTable evaluates a ground or finite-column condition on each
// table row. Conditions referring to interval columns require another path.
func (p *FiniteIntervalProduct) FilterInterpretedTable(condition ast.ASTHandle, theory *rewriter.TheoryRewriter) error {
	if err := p.open(); err != nil {
		return err
	}
	if theory == nil {
		return ErrTableRelation
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
	for _, column := range p.intervalColumns {
		if used.Contains(uint32(column)) != (ast.ASTHandle{}) {
			return ErrTableRelation
		}
	}
	rows, err := p.table.Facts()
	if err != nil {
		return err
	}
	for _, row := range rows {
		accept, err := p.evaluateTableCondition(condition, theory, row)
		if err != nil {
			return err
		}
		if !accept {
			if _, err = p.table.Remove(row); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *FiniteIntervalProduct) evaluateTableCondition(condition ast.ASTHandle, theory *rewriter.TheoryRewriter, row Fact) (accepted bool, resultErr error) {
	m := p.manager.manager
	bindings := make([]ast.ASTHandle, len(p.sorts))
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	for local, global := range p.tableColumns {
		value, err := p.manager.DecodeValue(p.sorts[global], row[local])
		if err != nil {
			return false, err
		}
		refs = append(refs, value)
		handle, err := m.RefGet(value)
		if err != nil {
			return false, err
		}
		bindings[len(bindings)-1-global] = handle
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	refs = append(refs, out)
	if err = theory.RewriteWithBindings(condition, bindings, out); err != nil {
		return false, err
	}
	value, err := m.RefGet(out)
	if err != nil {
		return false, err
	}
	if m.IsTrue(value) {
		return true, nil
	}
	if m.IsFalse(value) {
		return false, nil
	}
	return false, ErrTableRelation
}
