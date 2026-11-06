package rel

func (t *LazySparseTable) UnionFrom(source, delta *LazySparseTable) error {
	if err := t.open(); err != nil || source == nil || source.open() != nil || t.engine != source.engine {
		return ErrLazySparse
	}
	if delta != nil && (delta.open() != nil || t.engine != delta.engine) {
		return ErrLazySparse
	}
	if !t.root.signature.equal(source.root.signature) || t.root.functional != source.root.functional ||
		delta != nil && (!t.root.signature.equal(delta.root.signature) || t.root.functional != delta.root.functional) {
		return ErrLazySparse
	}
	// Source lazy union forces source, target, then optional delta.
	sourceTable, err := source.finiteEval()
	if err != nil {
		return err
	}
	targetTable, err := t.finiteEval()
	if err != nil {
		return err
	}
	var deltaTable FiniteTable
	if delta != nil {
		deltaTable, err = delta.finiteEval()
		if err != nil {
			return err
		}
	}
	if t.root.signature.Len() == 0 {
		targetHash, targetOK := targetTable.(*HashTable)
		sourceHash, sourceOK := sourceTable.(*HashTable)
		var deltaHash *HashTable
		if deltaTable != nil {
			deltaHash, targetOK = deltaTable.(*HashTable)
		}
		if !targetOK || !sourceOK {
			return ErrLazySparse
		}
		return targetHash.Union(sourceHash, deltaHash)
	}
	targetSparse, targetOK := targetTable.(*SparseTable)
	sourceSparse, sourceOK := sourceTable.(*SparseTable)
	var deltaSparse *SparseTable
	if deltaTable != nil {
		deltaSparse, targetOK = deltaTable.(*SparseTable)
	}
	if !targetOK || !sourceOK {
		return ErrLazySparse
	}
	return targetSparse.Union(sourceSparse, deltaSparse)
}
