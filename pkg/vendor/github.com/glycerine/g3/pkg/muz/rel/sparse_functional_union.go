package rel

// UnionFunctional mirrors sparse_table_plugin::union_fn: rows with an existing
// non-functional key do not replace its functional suffix and are not deltas.
func (target *SparseTable) UnionFunctional(source, delta *SparseTable) (int, error) {
	if err := target.open(); err != nil {
		return 0, err
	}
	if err := source.open(); err != nil {
		return 0, err
	}
	if !sparseSameFunctionalSignature(target, source) || (delta != nil && !sparseSameFunctionalSignature(target, delta)) {
		return 0, ErrTable
	}
	facts, err := source.Facts()
	if err != nil {
		return 0, err
	}
	inserted := 0
	for _, fact := range facts {
		if target.functional != 0 {
			firstFunctional := target.signature.Len() - target.functional
			_, found, fetchErr := target.FetchFunctionalKey(fact[:firstFunctional])
			if fetchErr != nil {
				return inserted, fetchErr
			}
			if found {
				continue
			}
		}
		changed, addErr := target.Add(fact)
		if addErr != nil {
			return inserted, addErr
		}
		if !changed {
			continue
		}
		inserted++
		if delta != nil {
			if _, addErr = delta.Add(fact); addErr != nil {
				return inserted, addErr
			}
		}
	}
	return inserted, nil
}
