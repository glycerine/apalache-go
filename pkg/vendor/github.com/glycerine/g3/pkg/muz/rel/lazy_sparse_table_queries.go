package rel

// Source: src/muz/rel/dl_lazy_table.cpp lazy_table::empty,
// lazy_table::remove_facts, and dl_lazy_table.h size metadata.

// Empty forces the deferred root and asks the concrete sparse/nullary table
// whether it owns any rows.
func (t *LazySparseTable) Empty() (bool, error) {
	table, err := t.finiteEval()
	if err != nil {
		return false, err
	}
	rows, err := table.Len()
	return rows == 0, err
}

// RemoveFacts preserves the source sequential bulk-removal semantics. A
// derived root is forced before mutation, so a shared deferred source remains
// independently owned.
func (t *LazySparseTable) RemoveFacts(facts []Fact) error {
	if err := t.open(); err != nil {
		return err
	}
	if t.root.signature.Len() == 0 {
		table, err := t.finiteEval()
		if err != nil {
			return err
		}
		hash, ok := table.(*HashTable)
		if !ok {
			return ErrLazySparse
		}
		return hash.RemoveFacts(facts)
	}
	table, err := t.eval()
	if err != nil {
		return err
	}
	_, err = table.RemoveFacts(facts)
	return err
}

// KnowsExactSize is deliberately false even after forcing, matching the
// lazy_table source contract rather than exposing the concrete table state.
func (t *LazySparseTable) KnowsExactSize() (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	return false, nil
}
