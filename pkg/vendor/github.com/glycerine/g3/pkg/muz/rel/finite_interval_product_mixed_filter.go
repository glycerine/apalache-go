package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp, mixed table/inner
// filter_interpreted_fn path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpreted dispatches a condition to the source table-only,
// interval-only, or mixed filtering path. The table path borrows theory.
func (p *FiniteIntervalProduct) FilterInterpreted(condition ast.ASTHandle, theory *rewriter.TheoryRewriter) error {
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
	for i := uint32(0); i < used.MaxFoundVarIndexPlusOne(); i++ {
		if s := used.Contains(i); s != (ast.ASTHandle{}) && s != p.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	tableUsed, innerUsed := false, false
	for _, column := range p.tableColumns {
		tableUsed = tableUsed || used.Contains(uint32(column)) != (ast.ASTHandle{})
	}
	for _, column := range p.intervalColumns {
		innerUsed = innerUsed || used.Contains(uint32(column)) != (ast.ASTHandle{})
	}
	if !innerUsed {
		return p.FilterInterpretedTable(condition, theory)
	}
	if !tableUsed {
		return p.FilterInterpretedInner(condition)
	}
	return p.filterInterpretedMixed(condition, used)
}

func (p *FiniteIntervalProduct) filterInterpretedMixed(condition ast.ASTHandle, used *ast.ASTUsedVars) (resultErr error) {
	mask := make([]bool, len(p.sorts))
	for _, column := range p.tableColumns {
		mask[column] = true
	}
	fresh, err := NewFiniteIntervalProduct(p.manager, p.sorts, mask)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, fresh.Close()) }()
	rows, err := p.table.Facts()
	if err != nil {
		return err
	}
	// The source groups by used table columns and the old relation index.
	groups := make(map[string]int)
	for _, row := range rows {
		oldIndex := uint64(row[len(row)-1])
		if oldIndex >= uint64(len(p.intervals)) || p.intervals[oldIndex] == nil {
			return ErrTableRelation
		}
		group := make(Fact, 0, len(p.tableColumns)+1)
		for local, global := range p.tableColumns {
			if used.Contains(uint32(global)) != (ast.ASTHandle{}) {
				group = append(group, row[local])
			}
		}
		group = append(group, Element(oldIndex))
		groupKey := factKey(group)
		if index, found := groups[groupKey]; found {
			if index >= 0 {
				row[len(row)-1] = Element(index)
				if err = fresh.table.EnsureFact(row); err != nil {
					return err
				}
			}
			continue
		}
		inner, err := p.filterMixedInner(condition, used, row, p.intervals[oldIndex])
		if err != nil {
			return err
		}
		if inner.Empty() {
			groups[groupKey] = -1
			continue
		}
		index := len(fresh.intervals)
		if err = fresh.storeInner(row, inner); err != nil {
			return err
		}
		groups[groupKey] = index
	}
	oldTable := p.table
	p.table, p.intervals, p.available = fresh.table, fresh.intervals, fresh.available
	fresh.table, fresh.intervals, fresh.available = nil, nil, nil
	return oldTable.Close()
}

func (p *FiniteIntervalProduct) filterMixedInner(condition ast.ASTHandle, used *ast.ASTUsedVars, row Fact, original *IntervalRelation) (_ *IntervalRelation, resultErr error) {
	m := p.manager.manager
	bindings := make([]ast.ASTHandle, len(p.sorts))
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	for local, global := range p.intervalColumns {
		variable, err := m.MkVar(uint32(local), p.sorts[global])
		if err != nil {
			return nil, err
		}
		ref, err := m.NewRef(variable)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
		bindings[global] = variable
	}
	for local, global := range p.tableColumns {
		if used.Contains(uint32(global)) == (ast.ASTHandle{}) {
			continue
		}
		ref, err := p.manager.DecodeValue(p.sorts[global], row[local])
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
		bindings[global], err = m.RefGet(ref)
		if err != nil {
			return nil, err
		}
	}
	subst, err := rewriter.NewVarSubstitution(m, false)
	if subst != nil {
		defer func() { resultErr = errors.Join(resultErr, subst.Close()) }()
	}
	if err != nil {
		return nil, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	refs = append(refs, out)
	if err = subst.Substitute(condition, bindings, out); err != nil {
		return nil, err
	}
	filtered, err := m.RefGet(out)
	if err != nil {
		return nil, err
	}
	inner := cloneIntervalRelation(original)
	if err = inner.FilterInterpreted(m, filtered); err != nil {
		return nil, err
	}
	return inner, nil
}
