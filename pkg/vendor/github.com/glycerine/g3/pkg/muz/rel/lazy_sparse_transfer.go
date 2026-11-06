package rel

// transferLazySparseSource mirrors lazy_table_ref::release_table for mutating
// force nodes. The result node owns the concrete table before releasing its
// child edge, so a refused release or mutator can be retried without losing the
// packed storage.
func (e *LazySparseEngine) transferLazySparseSource(node *lazySparseNode, child **lazySparseNode) (*SparseTable, error) {
	result := node.evaluated
	if result == nil {
		if child == nil || *child == nil {
			return nil, ErrLazySparse
		}
		var err error
		result, err = e.eval(*child)
		if err != nil {
			return nil, err
		}
		if (*child).references > 1 {
			// Transformer nodes may share a loaded predicate root. Preserve that
			// source owner and give the mutator an independent packed table.
			result, err = result.Clone()
			if err != nil {
				return nil, err
			}
		} else {
			(*child).evaluated = nil
		}
		node.evaluated = result
	}
	node.retryForce = true
	if child != nil && *child != nil {
		if err := releaseLazySparseNode(*child); err != nil {
			return nil, err
		}
		*child = nil
	}
	return result, nil
}
