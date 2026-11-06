package rel

// Source: src/muz/rel/dl_lazy_table.cpp lazy_table::complement accepts a null
// functional-column vector for ordinary tables and forwards to the concrete
// sparse plugin.

// Complement returns the ordinary finite-domain complement. Functional lazy
// sparse tables require the explicit suffix supplied to ComplementFunctional.
func (t *LazySparseTable) Complement() (*LazySparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if t.root.functional != 0 {
		return nil, ErrLazySparse
	}
	return t.ComplementFunctional(nil)
}
