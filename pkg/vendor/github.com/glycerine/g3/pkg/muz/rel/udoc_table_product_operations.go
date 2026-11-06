package rel

// Source: dl_product_relation.cpp diagonal aligned_union_fn and mutator_fn,
// with dl_sieve_relation.cpp's concrete DOC mask dispatch. Cross-component
// refinements require both directional unions; finite<-DOC is unavailable.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *UDocTableProduct) EmptyLike() (*UDocTableProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	a := newUDocTableProductAttempt(p.manager)
	inner, err := p.doc.inner.factory.NewEmpty(p.doc.inner.sorts)
	a.product.doc, err = finishUDocSieve(p.doc.sorts, p.doc.innerColumns, inner, err)
	if err == nil && p.table != nil {
		a.product.table, err = emptyFiniteLike(p.table)
		a.product.tableColumns = append([]bool(nil), p.tableColumns...)
	}
	return a.finish(err)
}

func (p *UDocTableProduct) sameLayout(other *UDocTableProduct) bool {
	if p.open() != nil || other.open() != nil || p.manager != other.manager || !p.doc.inner.sameShape(other.doc.inner) || len(p.doc.sorts) != len(other.doc.sorts) {
		return false
	}
	leftMask, rightMask := productTableMask(p.doc.sorts, p.tableColumns), productTableMask(other.doc.sorts, other.tableColumns)
	for i, sort := range p.doc.sorts {
		if sort != other.doc.sorts[i] || p.doc.innerColumns[i] != other.doc.innerColumns[i] {
			return false
		}
		if p.table != nil && other.table != nil && leftMask[i] != rightMask[i] {
			return false
		}
	}
	if p.table != nil && other.table != nil && rawUDocProductFiniteKind(p.table) != rawUDocProductFiniteKind(other.table) {
		return false
	}
	return true
}

func (p *UDocTableProduct) AddFact(values []ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if _, err := p.doc.AddFact(values); err != nil {
		return err
	}
	if p.table == nil {
		return nil
	}
	mask := productTableMask(p.doc.sorts, p.tableColumns)
	sorts := productTableInnerSorts(p.doc.sorts, mask)
	selected := make([]ast.ASTHandle, 0, len(sorts))
	for column, include := range mask {
		if include {
			selected = append(selected, values[column])
		}
	}
	fact, err := p.manager.EncodeFact(sorts, selected)
	if err != nil {
		return err
	}
	_, err = p.table.Add(fact)
	return err
}

// UnionFrom requires aligned concrete components. Compiler delta registers
// are distinct owners; each receives the source's fresh component delta by
// union, preserving any facts already accumulated by earlier instructions.
func (p *UDocTableProduct) UnionFrom(source, delta *UDocTableProduct) (resultErr error) {
	if !p.sameLayout(source) || delta != nil && (!p.sameLayout(delta) || delta == p || delta == source) {
		return ErrUDocTableProduct
	}
	// product_relation::unaligned_union_fn converts both sides to their common
	// component specification. A missing finite component is full for a
	// nonempty product and empty for a newly created empty product.
	reference := p
	if reference.table == nil {
		reference = source
	}
	if reference.table == nil && delta != nil {
		reference = delta
	}
	if reference.table != nil {
		for _, product := range []*UDocTableProduct{p, source, delta} {
			if product == nil || product.table != nil {
				continue
			}
			table, err := emptyFiniteLike(reference.table)
			if err != nil {
				return err
			}
			empty, err := product.doc.Empty()
			if err == nil && !empty {
				signature, signatureErr := table.Signature()
				if signatureErr != nil {
					err = signatureErr
				} else {
					err = fillTotalTable(table, signature)
				}
			}
			if err != nil {
				return errors.Join(err, table.Close())
			}
			product.table = table
			product.tableColumns = append([]bool(nil), reference.tableColumns...)
		}
	}
	if !p.sameLayout(source) || delta != nil && !p.sameLayout(delta) {
		return ErrUDocTableProduct
	}
	if delta == nil {
		if err := p.doc.inner.UnionFrom(source.doc.inner, nil); err != nil {
			return err
		}
		if p.table == nil {
			return nil
		}
		return finiteTableUnion(p.table, source.table, nil)
	}
	a := newUDocTableProductAttempt(p.manager)
	var err error
	a.product, err = p.EmptyLike()
	if err != nil {
		return a.discard(err)
	}
	defer func() { resultErr = a.discard(resultErr) }()
	if err = p.doc.inner.UnionFrom(source.doc.inner, a.product.doc.inner); err != nil {
		return err
	}
	if err = delta.doc.inner.UnionFrom(a.product.doc.inner, nil); err != nil {
		return err
	}
	if p.table != nil {
		if err = finiteTableUnion(p.table, source.table, a.product.table); err != nil {
			return err
		}
		return finiteTableUnion(delta.table, a.product.table, nil)
	}
	return nil
}

func (p *UDocTableProduct) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if column >= uint32(len(p.doc.sorts)) {
		return ErrUDocTableProduct
	}
	mask := productTableMask(p.doc.sorts, p.tableColumns)
	innerColumns := productTableColumnMap(mask)
	var encoded Element
	if p.table != nil && innerColumns[column] >= 0 {
		var err error
		encoded, err = p.manager.EncodeValue(p.doc.sorts[column], value)
		if err != nil {
			return err
		}
	}
	if inner := p.doc.sigToInner[column]; inner >= 0 {
		if err := p.doc.inner.FilterEqual(uint32(inner), value); err != nil {
			return err
		}
	}
	if p.table == nil {
		return nil
	}
	if innerColumns[column] < 0 {
		return nil
	}
	return finiteTableFilterEqual(p.table, encoded, innerColumns[column])
}

func (p *UDocTableProduct) FilterIdentical(columns []uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	if len(columns) == 0 {
		return nil
	}
	var selected []uint32
	mask := productTableMask(p.doc.sorts, p.tableColumns)
	innerColumns := productTableColumnMap(mask)
	var finite []int
	for _, column := range columns {
		if column >= uint32(len(p.doc.sorts)) || p.doc.sorts[column] != p.doc.sorts[columns[0]] {
			return ErrUDocTableProduct
		}
		if innerColumns[column] >= 0 {
			finite = append(finite, innerColumns[column])
		}
		if inner := p.doc.sigToInner[column]; inner >= 0 {
			selected = append(selected, uint32(inner))
		}
	}
	if len(selected) >= 2 {
		if err := p.doc.inner.FilterIdentical(selected); err != nil {
			return err
		}
	}
	if len(finite) < 2 {
		return nil
	}
	if p.table == nil {
		return nil
	}
	return finiteTableFilterIdentical(p.table, finite)
}
