package rel

import "errors"

// Clone mirrors lazy_table::clone: force the current operation root, clone its
// concrete owner, and wrap the clone as an independently owned lazy base node.
func (t *LazySparseTable) Clone() (*LazySparseTable, error) {
	table, err := t.finiteEval()
	if err != nil {
		return nil, err
	}
	clone, err := table.CloneFinite()
	if err != nil {
		return nil, err
	}
	switch concrete := clone.(type) {
	case *SparseTable:
		result, adoptErr := t.engine.adoptSparse(concrete)
		if adoptErr != nil {
			return nil, errors.Join(adoptErr, concrete.Close())
		}
		return result, nil
	case *HashTable:
		signature, signatureErr := concrete.Signature()
		if signatureErr != nil {
			return nil, errors.Join(signatureErr, concrete.Close())
		}
		return t.engine.adopt(&lazySparseNode{kind: LazyTableBase, signature: signature, nullary: concrete}), nil
	default:
		return nil, errors.Join(ErrLazySparse, clone.Close())
	}
}
