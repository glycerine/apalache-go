package rel

import "fmt"

// FilterIdentical appends the source lazy_table_filter_identical node without
// forcing the packed sparse source.
func (t *LazySparseTable) FilterIdentical(columns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	for _, column := range columns {
		if column < 0 || column >= t.root.signature.Len() {
			return fmt.Errorf("%w: identical-filter column out of range", ErrLazySparse)
		}
	}
	return t.replaceLazySparseRoot(&lazySparseNode{
		kind: LazyTableFilterIdentical, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional,
		left: t.root, columns1: append([]int(nil), columns...),
	})
}

func evalLazySparseIdentical(engine *LazySparseEngine, node *lazySparseNode) (*SparseTable, error) {
	result, err := engine.transferLazySparseSource(node, &node.left)
	if err != nil {
		return nil, err
	}
	if err = result.FilterIdentical(node.columns1); err != nil {
		return nil, err
	}
	return result, nil
}
