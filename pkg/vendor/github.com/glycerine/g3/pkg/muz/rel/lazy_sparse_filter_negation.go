package rel

// FilterByNegation appends the source lazy_table_filter_by_negation node
// without forcing either packed sparse operand.
func (t *LazySparseTable) FilterByNegation(source *LazySparseTable, targetColumns, sourceColumns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || source.engine != t.engine {
		return ErrLazySparse
	}
	if err := validateColumns(t.root.signature, source.root.signature, targetColumns, sourceColumns); err != nil {
		return err
	}
	return t.replaceLazySparseRoot(&lazySparseNode{
		kind: LazyTableFilterByNegation, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional,
		left: t.root, right: source.root, columns1: append([]int(nil), targetColumns...), columns2: append([]int(nil), sourceColumns...),
	})
}

func evalLazySparseNegation(engine *LazySparseEngine, node *lazySparseNode) (*SparseTable, error) {
	target, err := engine.transferLazySparseSource(node, &node.left)
	if err != nil {
		return nil, err
	}
	// SparseNegatedJoinPlan consumes raw [all-left, all-right] columns. Lazy
	// functional joins reorder those columns to [left-data, right-data,
	// left-functional, right-functional], so use the plan only when no remap is
	// required and fall back through the materialized lazy join otherwise.
	if node.right.kind == LazyTableJoin && node.right.left.signature.Len() != 0 && node.right.right.signature.Len() != 0 &&
		node.right.left.functional == 0 && node.right.right.functional == 0 {
		first, firstErr := engine.eval(node.right.left)
		if firstErr != nil {
			return nil, firstErr
		}
		second, secondErr := engine.eval(node.right.right)
		if secondErr != nil {
			return nil, secondErr
		}
		plan, planErr := NewSparseNegatedJoinPlan(target, first, second, node.columns1, node.columns2, node.right.columns1, node.right.columns2)
		if planErr == nil {
			_, planErr = plan.Apply(target, first, second)
			if planErr != nil {
				return nil, planErr
			}
			return target, nil
		}
	}
	negative, err := engine.eval(node.right)
	if err != nil {
		return nil, err
	}
	if err = target.FilterNegation(negative, node.columns1, node.columns2); err != nil {
		return nil, err
	}
	return target, nil
}
