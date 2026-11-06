package rel

// Source: src/muz/rel/dl_lazy_table.h lazy_table_plugin::can_handle_signature
// delegates to src/muz/rel/dl_sparse_table.h sparse_table_plugin.

// CanHandleSignature reports the exact concrete sparse capability: the
// deferred plugin accepts every nonempty finite signature and declines the
// nullary signature so relation-manager selection can fall back to Hash.
func (f *LazySparseFiniteFactory) CanHandleSignature(signature Signature) bool {
	return f != nil && f.open() == nil && signature.Len() != 0
}
