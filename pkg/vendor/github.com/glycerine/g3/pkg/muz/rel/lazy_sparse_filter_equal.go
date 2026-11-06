package rel

import "fmt"

// FilterEqual appends the source lazy_table_filter_equal node without forcing
// the packed sparse source.
func (t *LazySparseTable) FilterEqual(value Element, column int) error {
	if err := t.open(); err != nil {
		return err
	}
	if column < 0 || column >= t.root.signature.Len() || uint64(value) >= t.root.signature.domains[column] {
		return fmt.Errorf("%w: equal-filter value or column out of range", ErrLazySparse)
	}
	return t.replaceLazySparseRoot(&lazySparseNode{
		kind: LazyTableFilterEqual, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional,
		left: t.root, value: value, column: column,
	})
}

func evalLazySparseEqual(engine *LazySparseEngine, node *lazySparseNode) (*SparseTable, error) {
	result, err := engine.transferLazySparseSource(node, &node.left)
	if err != nil {
		return nil, err
	}
	if err = result.FilterEqual(node.value, node.column); err != nil {
		return nil, err
	}
	return result, nil
}

func evalLazySparseEqualProject(engine *LazySparseEngine, filter *lazySparseNode) (*SparseTable, error) {
	// Source lazy_table_project::force evaluates the filter node, not its
	// source. A prior mutating force may already have transferred the source
	// table and cleared filter.left.
	source, err := engine.eval(filter)
	if err != nil {
		return nil, err
	}
	return source.SelectEqualAndProject(filter.value, filter.column)
}
