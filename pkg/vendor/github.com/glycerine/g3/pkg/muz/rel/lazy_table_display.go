package rel

// Source: src/muz/rel/dl_lazy_table.cpp lazy_table::begin/end and
// src/muz/rel/dl_base.cpp table_base::display.

import "io"

// Display forces the deferred root and forwards source table formatting to
// the concrete owner used by the lazy iterator boundary.
func (t *LazyTable) Display(out io.Writer) error {
	table, err := t.Eval()
	if err != nil {
		return err
	}
	return table.Display(out)
}
