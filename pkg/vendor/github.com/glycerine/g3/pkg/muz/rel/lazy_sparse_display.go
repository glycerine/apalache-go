package rel

// Source: src/muz/rel/dl_lazy_table.cpp lazy_table::begin/end and
// src/muz/rel/dl_base.cpp table_base::display.

import "io"

func displayLazySparseFinite(table FiniteTable, out io.Writer) error {
	switch concrete := table.(type) {
	case *SparseTable:
		return concrete.Display(out)
	case *HashTable:
		return concrete.Display(out)
	default:
		return ErrLazySparse
	}
}
