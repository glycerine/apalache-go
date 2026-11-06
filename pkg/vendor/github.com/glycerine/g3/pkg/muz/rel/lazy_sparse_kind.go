package rel

// Source: src/muz/rel/dl_lazy_table.h lazy_table_ref::kind.

// Kind exposes the deferred node specialization used by source fusion
// dispatch. Like LazyTable.Kind, an unavailable handle reports the base kind.
func (t *LazySparseTable) Kind() LazyTableKind {
	if t.open() != nil {
		return LazyTableBase
	}
	return t.root.kind
}
