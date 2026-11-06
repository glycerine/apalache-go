package rel

// fetchInnerKey follows finite_product_relation's functional sparse-table
// lookup: the table columns form the key and the single functional suffix is
// the owned inner-relation index. In particular, an all-interval product has
// an empty table key rather than a fabricated full row.
func (p *FiniteIntervalProduct) fetchInnerKey(key Fact) (bool, error) {
	if err := p.open(); err != nil || len(key) == 0 {
		if err != nil {
			return false, err
		}
		return false, ErrTableRelation
	}
	suffix, found, err := p.table.FetchFunctionalKey(key[:len(key)-1])
	if err != nil || !found {
		return found, err
	}
	if len(suffix) != 1 {
		return false, ErrTableRelation
	}
	key[len(key)-1] = suffix[0]
	return true, nil
}
