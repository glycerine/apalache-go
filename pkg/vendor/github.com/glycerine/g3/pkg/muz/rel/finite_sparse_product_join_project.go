package rel

// Source: src/muz/rel/dl_relation_manager.cpp default_relation_join_project_fn.

// JoinProject composes the source relation-manager join and projection
// fallbacks while releasing the temporary joined product.
func (p *FiniteSparseProduct) JoinProject(right *FiniteSparseProduct, leftColumns, rightColumns, removed []int) (*FiniteSparseProduct, error) {
	joined, err := p.Join(right, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	defer joined.Close()
	return joined.Project(removed)
}
