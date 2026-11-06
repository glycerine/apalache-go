package rel

func lazySparseRenameMetadata(source *lazySparseNode, cycle []int) (Signature, error) {
	signature, err := lazyRenameSignature(source.signature, cycle)
	if err != nil {
		return Signature{}, err
	}
	if source.functional != 0 && len(cycle) != 0 {
		first := source.signature.Len() - source.functional
		functionalSide := cycle[0] >= first
		for _, column := range cycle[1:] {
			if (column >= first) != functionalSide {
				return Signature{}, ErrLazySparse
			}
		}
	}
	return signature, nil
}

func (t *LazySparseTable) RenameCycle(cycle []int) (*LazySparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	signature, err := lazySparseRenameMetadata(t.root, cycle)
	if err != nil {
		return nil, err
	}
	return t.engine.adopt(&lazySparseNode{
		kind: LazyTableRename, signature: signature, functional: t.root.functional,
		left: t.root, columns1: append([]int(nil), cycle...),
	}), nil
}
