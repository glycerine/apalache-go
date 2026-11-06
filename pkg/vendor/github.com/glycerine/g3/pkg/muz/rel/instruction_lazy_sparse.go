package rel

import "errors"

// lazySparseInstructionResult constructs compiler temporaries in the same
// engine as an operand. Explicit lazy-sparse predicates are non-nullary, while
// compiler projections may use the internal Hash-backed nullary node.
func lazySparseInstructionResult(source *LazySparseTable, signature Signature, functional int) (*LazySparseTable, error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	if signature.Len() != 0 {
		return source.engine.New(signature, functional)
	}
	return source.engine.adopt(&lazySparseNode{
		kind: LazyTableBase, signature: Signature{domains: signature.Domains()}, nullary: NewHashTable(signature),
	}), nil
}

func lazySparseInstructionJoinProject(first, second *LazySparseTable, left, right, removed []int) (_ *LazySparseTable, resultErr error) {
	joined, err := first.Join(second, left, right)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, joined.Close()) }()
	return joined.Project(removed)
}

func lazySparseInstructionSelectProject(source *LazySparseTable, value Element, column int) (_ *LazySparseTable, resultErr error) {
	if err := source.open(); err != nil || column < 0 || column >= source.root.signature.Len() || uint64(value) >= source.root.signature.domains[column] {
		return nil, ErrLazySparse
	}
	filtered := source.engine.adopt(&lazySparseNode{
		kind: LazyTableFilterEqual, signature: Signature{domains: source.root.signature.Domains()}, functional: source.root.functional,
		left: source.root, value: value, column: column,
	})
	defer func() { resultErr = errors.Join(resultErr, filtered.Close()) }()
	return filtered.Project([]int{column})
}
