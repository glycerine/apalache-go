package rel

// fetchInnerKey uses the concrete sparse functional index's nonfunctional
// prefix lookup. Checked and lazy explanation indexes keep their own verified
// FetchFact implementations.
func (p *FiniteExplanationProduct) fetchInnerKey(key Fact) (bool, error) {
	if err := p.open(); err != nil || len(key) == 0 {
		if err != nil {
			return false, err
		}
		return false, ErrTableRelation
	}
	if sparse, ok := p.table.(*SparseTable); ok {
		suffix, found, err := sparse.FetchFunctionalKey(key[:len(key)-1])
		if err != nil || !found {
			return found, err
		}
		if len(suffix) != 1 {
			return false, ErrTableRelation
		}
		key[len(key)-1] = suffix[0]
		return true, nil
	}
	return p.table.FetchFact(key)
}
