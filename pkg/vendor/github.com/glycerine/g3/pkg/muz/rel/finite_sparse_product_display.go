package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp display methods and
// src/muz/rel/dl_base.cpp table_base::display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// Display writes the outer table and each live nested sparse relation in
// source index order, collecting unused and empty inner relations first.
func (p *FiniteSparseProduct) Display(out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrTableRelation
	}
	if err := p.collectGarbage(true); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "finite_product_relation:\n table:\n"); err != nil {
		return err
	}
	if err := p.table.Display(out); err != nil {
		return err
	}
	for index, inner := range p.inner {
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

// DisplayTuples writes expanded finite tuples using Datalog argument names
// and finite-sort constant spellings.
func (p *FiniteSparseProduct) DisplayTuples(context *base.DatalogContext, predicate ast.ASTHandle, out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	flat, err := p.flatten()
	if err != nil {
		return err
	}
	defer flat.Close()
	table, err := NewTableRelation(p.manager, p.sorts)
	if err != nil {
		return err
	}
	defer table.Close()
	facts, err := flat.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if _, err = table.AddTableFact(fact); err != nil {
			return err
		}
	}
	return table.DisplayTuples(context, predicate, out)
}
