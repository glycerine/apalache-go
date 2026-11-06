package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp
// finite_product_relation::display and the size-estimate methods in
// dl_finite_product_relation.h, specialized to the manager-owned explanation
// product registered by dl_mk_explanations.cpp.

import (
	"fmt"
	"io"
)

// SizeEstimateRows reports the product's outer-table row estimate.
func (p *FiniteExplanationProduct) SizeEstimateRows() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.table.SizeEstimateRows()
}

// SizeEstimateBytes reports the product's outer-table storage estimate.
func (p *FiniteExplanationProduct) SizeEstimateBytes() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.table.SizeEstimateBytes()
}

// Display writes the outer table and each live explanation relation in source
// index order, collecting unreferenced and empty inner owners first.
func (p *FiniteExplanationProduct) Display(out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrTableRelation
	}
	if err := p.collectEmptyExplanationRows(); err != nil {
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
