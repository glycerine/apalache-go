package rel

// Source: dl_lazy_table.cpp mutating force nodes transfer the evaluated table
// from m_src/m_tgt and then clear that child reference before applying their
// mutator. A Go child release may refuse, so the transferred table remains
// owned by node while the child edge is retried.
func (e *LazyTableEngine) transferLazySource(node *lazyTableNode, child **lazyTableNode) (*HashTable, error) {
	result := node.evaluated
	if result == nil {
		if child == nil || *child == nil {
			return nil, ErrLazyTable
		}
		var err error
		result, err = e.evaluate(*child)
		if err != nil {
			return nil, err
		}
		(*child).evaluated = nil
		node.evaluated = result
	}
	node.retryForce = true
	if child != nil && *child != nil {
		consumed, err := releaseLazyTableNode(*child)
		if consumed {
			*child = nil
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
