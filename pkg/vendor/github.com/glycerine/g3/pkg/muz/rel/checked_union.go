package rel

func (r *CheckedRelation) Union(source, delta *CheckedRelation) error {
	var pd *HashTable
	var sd *SparseTable
	if delta != nil {
		pd = delta.primary
		sd = delta.checker
	}
	if e := r.primary.Union(source.primary, pd); e != nil {
		return e
	}
	if e := r.checker.Union(source.checker, sd); e != nil {
		return e
	}
	if e := r.verify("union"); e != nil {
		return e
	}
	if delta != nil {
		return delta.verify("union delta")
	}
	return nil
}
