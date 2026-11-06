package rel

// Source: src/muz/rel/dl_product_relation.cpp filter and union component
// dispatch, instantiated by tr_sparse + interval_relation sieve components.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

func (p *SparseIntervalSieveProduct) finiteColumn(column int) int {
	for local, outer := range p.finiteColumns {
		if outer == column {
			return local
		}
	}
	return -1
}

func (p *SparseIntervalSieveProduct) FilterEqual(column int, value ast.ASTHandle) error {
	if err := p.open(); err != nil || column < 0 || column >= len(p.sorts) {
		if err != nil {
			return err
		}
		return ErrSparseIntervalProduct
	}
	if local := p.finiteColumn(column); local >= 0 {
		encoded, err := p.manager.EncodeValue(p.sorts[column], value)
		if err != nil {
			return err
		}
		return p.finite.FilterEqual(encoded, local)
	}
	return p.interval.FilterEqual(uint32(column), value)
}

func (p *SparseIntervalSieveProduct) FilterIdentical(columns []int) error {
	if err := p.open(); err != nil {
		return err
	}
	finite := make([]int, 0, len(columns))
	interval := make([]uint32, 0, len(columns))
	for _, column := range columns {
		if column < 0 || column >= len(p.sorts) {
			return ErrSparseIntervalProduct
		}
		if local := p.finiteColumn(column); local >= 0 {
			finite = append(finite, local)
		} else {
			interval = append(interval, uint32(column))
		}
	}
	if err := p.finite.FilterIdentical(finite); err != nil {
		return err
	}
	return p.interval.FilterIdentical(interval)
}

func (p *SparseIntervalSieveProduct) finiteInterpreted(condition ast.ASTHandle, theory *astrewriter.TheoryRewriter) (resultErr error) {
	used, err := ast.NewASTUsedVars(p.manager.manager)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil || uint64(used.MaxFoundVarIndexPlusOne()) > uint64(len(p.sorts)) {
		if err != nil {
			return err
		}
		return ErrSparseIntervalProduct
	}
	for outer := range p.sorts {
		if used.Contains(uint32(outer)) != (ast.ASTHandle{}) && p.finiteColumn(outer) < 0 {
			return nil // a condition spanning the other component is identity here
		}
	}
	bindings := make([]ast.ASTHandle, len(p.sorts))
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.manager.manager.DelRef(&refs[i]))
		}
	}()
	for local, outer := range p.finiteColumns {
		if used.Contains(uint32(outer)) == (ast.ASTHandle{}) {
			continue
		}
		variable, e := p.manager.manager.MkVar(uint32(local), p.sorts[outer])
		if e != nil {
			return e
		}
		ref, e := p.manager.manager.NewRef(variable)
		if e != nil {
			return e
		}
		refs = append(refs, ref)
		bindings[outer] = variable
	}
	substitution, err := astrewriter.NewVarSubstitution(p.manager.manager, false)
	if substitution != nil {
		defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	}
	if err != nil {
		return err
	}
	out, err := p.manager.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	refs = append(refs, out)
	if err = substitution.Substitute(condition, bindings, out); err != nil {
		return err
	}
	rewritten, err := p.manager.manager.RefGet(out)
	if err != nil {
		return err
	}
	return p.finite.FilterInterpreted(p.manager, p.finiteSorts, theory, rewritten)
}

func (p *SparseIntervalSieveProduct) FilterInterpreted(condition ast.ASTHandle, theory *astrewriter.TheoryRewriter) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.finiteInterpreted(condition, theory); err != nil {
		return err
	}
	return p.interval.FilterInterpreted(condition)
}

func (p *SparseIntervalSieveProduct) sameLayout(other *SparseIntervalSieveProduct) bool {
	if other == nil || p.manager != other.manager || len(p.sorts) != len(other.sorts) || len(p.finiteColumns) != len(other.finiteColumns) {
		return false
	}
	for i := range p.sorts {
		if p.sorts[i] != other.sorts[i] {
			return false
		}
	}
	for i := range p.finiteColumns {
		if p.finiteColumns[i] != other.finiteColumns[i] {
			return false
		}
	}
	return true
}

func (p *SparseIntervalSieveProduct) Union(source *SparseIntervalSieveProduct, widen bool) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if source == nil || source.open() != nil || !p.sameLayout(source) {
		return false, ErrSparseIntervalProduct
	}
	before, err := p.finite.Len()
	if err != nil {
		return false, err
	}
	if err = p.finite.Union(source.finite, nil); err != nil {
		return false, err
	}
	after, err := p.finite.Len()
	if err != nil {
		return false, err
	}
	intervalChanged, err := p.interval.inner.Union(source.interval.inner, widen)
	return after != before || intervalChanged, err
}
