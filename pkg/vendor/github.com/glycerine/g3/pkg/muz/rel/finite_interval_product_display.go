package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp
// finite_product_relation::display and the size-estimate methods in
// dl_finite_product_relation.h.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// SizeEstimateRows reports the source product's outer-table row estimate.
func (p *FiniteIntervalProduct) SizeEstimateRows() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.table.SizeEstimateRows()
}

// SizeEstimateBytes reports the source product's outer-table storage estimate.
func (p *FiniteIntervalProduct) SizeEstimateBytes() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.table.SizeEstimateBytes()
}

// Display writes the outer table and every live interval relation in source
// index order. Source display first removes unreferenced and empty inners.
func (p *FiniteIntervalProduct) Display(out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrTableRelation
	}
	if err := p.garbageCollect(true); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "finite_product_relation:\n table:\n"); err != nil {
		return err
	}
	if err := p.table.Display(out); err != nil {
		return err
	}
	for index, inner := range p.intervals {
		if inner == nil {
			continue
		}
		if _, err := fmt.Fprintf(out, " inner relation %d:\n", index); err != nil {
			return err
		}
		if err := inner.Display(out); err != nil {
			return err
		}
	}
	return nil
}

// DisplayTuples follows the non-table inner-plugin branch: interval relations
// cannot enumerate exact tuples, so source writes the heading and delegates to
// the structural product display.
func (p *FiniteIntervalProduct) DisplayTuples(context *base.DatalogContext, predicate ast.ASTHandle, out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if context == nil || out == nil {
		return ErrTableRelation
	}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	if manager != p.manager.manager {
		return ErrTableRelation
	}
	kind, err := manager.GetKind(predicate)
	if err != nil {
		return err
	}
	if kind != ast.ASTFuncDecl {
		return ErrTableRelation
	}
	arity, err := manager.GetArity(predicate)
	if err != nil {
		return err
	}
	if int(arity) != len(p.sorts) {
		return ErrTableRelation
	}
	for index := range p.sorts {
		sort, getErr := manager.GetDomain(predicate, uint32(index))
		if getErr != nil {
			return getErr
		}
		if sort != p.sorts[index] {
			return ErrTableRelation
		}
	}
	name, err := manager.GetASTName(predicate)
	if err != nil {
		return err
	}
	nameText, err := manager.Symbols.Display(name)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "Tuples in %s: \n", nameText); err != nil {
		return err
	}
	return p.Display(out)
}
