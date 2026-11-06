package rel

// Source: src/muz/rel/dl_product_relation.cpp product_relation::display.

import (
	"fmt"
	"io"
)

// Display writes the source product header followed by each component in the
// requested representation order.
func (p *ExternalIntervalProduct) Display(out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrTableRelation
	}
	if _, err := fmt.Fprintln(out, "Product of the following relations:"); err != nil {
		return err
	}
	for _, kind := range p.kinds {
		switch kind {
		case "external_relation":
			if err := p.external.Display(out); err != nil {
				return err
			}
		case "interval_relation":
			if err := p.interval.Display(out); err != nil {
				return err
			}
		default:
			return ErrTableRelation
		}
	}
	return nil
}
