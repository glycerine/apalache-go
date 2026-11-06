package rel

import "errors"

// ComplementFunctional mirrors lazy_table::complement: force the deferred
// source, invoke the concrete table complement with its functional suffix,
// and return a new lazy base owner in the same engine.
func (t *LazySparseTable) ComplementFunctional(functionalSuffix Fact) (*LazySparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if t.root.signature.Len() == 0 {
		if len(functionalSuffix) != 0 {
			return nil, ErrLazySparse
		}
		table, err := t.finiteEval()
		if err != nil {
			return nil, err
		}
		hash, ok := table.(*HashTable)
		if !ok {
			return nil, ErrLazySparse
		}
		complement, err := hash.Complement()
		if err != nil {
			return nil, err
		}
		return t.engine.adopt(&lazySparseNode{kind: LazyTableBase, signature: t.root.signature, nullary: complement}), nil
	}
	table, err := t.eval()
	if err != nil {
		return nil, err
	}
	complement, err := table.ComplementFunctional(functionalSuffix)
	if err != nil {
		return nil, err
	}
	result, err := t.engine.adoptSparse(complement)
	if err != nil {
		return nil, errors.Join(err, complement.Close())
	}
	return result, nil
}
