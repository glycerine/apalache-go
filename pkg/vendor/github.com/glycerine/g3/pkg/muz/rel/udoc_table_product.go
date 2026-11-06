package rel

// Source: dl_product_relation.cpp product_relation storage, membership,
// componentwise projection and formula. This concrete product joins raw DOC
// with an ordinary finite table; it is not finite_product_relation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrUDocTableProduct = errors.New("muz/rel: invalid DOC/table product")

// UDocTableProduct owns two independent abstractions over the same signature.
// The DOC sieve pins every original sort, including its ignored Bool columns.
// The relation manager and DOC factory are borrowed and outlive this owner.
type UDocTableProduct struct {
	manager      *RelationManager
	doc          *UDocSieve
	table        FiniteTable
	tableColumns []bool // nil means every outer column belongs to the table.
}

func (p *UDocTableProduct) open() error {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.doc == nil || p.doc.open() != nil {
		return ErrUDocTableProduct
	}
	if p.table != nil {
		if p.tableColumns != nil && len(p.tableColumns) != len(p.doc.sorts) {
			return ErrUDocTableProduct
		}
		sorts := productTableInnerSorts(p.doc.sorts, productTableMask(p.doc.sorts, p.tableColumns))
		expected, err := tableRelationSignature(p.manager, sorts)
		if err != nil {
			return ErrUDocTableProduct
		}
		actual, err := p.table.Signature()
		if err != nil || !expected.equal(actual) {
			return ErrUDocTableProduct
		}
	}
	return nil
}

func (p *UDocTableProduct) Sorts() []ast.ASTHandle {
	if p.open() != nil {
		return nil
	}
	return p.doc.Sorts()
}

// Doc and Table return borrowed component owners for concrete source dispatch.
func (p *UDocTableProduct) Doc() *UDocSieve {
	if p.open() != nil {
		return nil
	}
	return p.doc
}
func (p *UDocTableProduct) Table() FiniteTable {
	if p.open() != nil {
		return nil
	}
	return p.table
}

// TableColumns returns the product mask; false columns are ignored by the
// finite table component, as in a source sieve_relation.
func (p *UDocTableProduct) TableColumns() []bool {
	if p.open() != nil || p.table == nil {
		return nil
	}
	return productTableMask(p.doc.sorts, p.tableColumns)
}

func (p *UDocTableProduct) Close() error {
	if p == nil {
		return nil
	}
	if p.table != nil {
		if err := p.table.Close(); err != nil {
			return err
		}
		p.table = nil
	}
	if p.doc != nil {
		if err := p.doc.Close(); err != nil {
			return err
		}
		p.doc = nil
	}
	p.manager = nil
	return nil
}

// Empty preserves source product_relation::empty: any empty component proves
// emptiness, but conflicting nonempty components are not intersected here.
func (p *UDocTableProduct) Empty() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	empty, err := p.doc.Empty()
	if err != nil || empty {
		return empty, err
	}
	if p.table == nil {
		return false, nil
	}
	count, err := p.table.Len()
	return count == 0, err
}

func (p *UDocTableProduct) ContainsFact(values []ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	found, err := p.doc.ContainsFact(values)
	if err != nil || !found {
		return found, err
	}
	if p.table == nil {
		return true, nil
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
		return false, err
	}
	return p.table.Contains(fact)
}

func (p *UDocTableProduct) Clone() (*UDocTableProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	a := newUDocTableProductAttempt(p.manager)
	var err error
	a.product.doc, err = p.doc.Clone()
	if err == nil && p.table != nil {
		a.product.table, err = p.table.CloneFinite()
		a.product.tableColumns = append([]bool(nil), p.tableColumns...)
	}
	return a.finish(err)
}

func (p *UDocTableProduct) Project(removed []uint32) (*UDocTableProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	a := newUDocTableProductAttempt(p.manager)
	var err error
	a.product.doc, err = p.doc.Project(removed)
	if err == nil && p.table != nil {
		mask := productTableMask(p.doc.sorts, p.tableColumns)
		removedSet := make(map[uint32]bool, len(removed))
		for _, column := range removed {
			removedSet[column] = true
		}
		var columns []int
		for column, inner := range productTableColumnMap(mask) {
			if inner >= 0 && removedSet[uint32(column)] {
				columns = append(columns, inner)
			}
		}
		a.product.tableColumns = make([]bool, 0, len(mask)-len(removed))
		for column, include := range mask {
			if !removedSet[uint32(column)] {
				a.product.tableColumns = append(a.product.tableColumns, include)
			}
		}
		a.product.table, err = finiteTableProject(p.table, columns)
	}
	return a.finish(err)
}
