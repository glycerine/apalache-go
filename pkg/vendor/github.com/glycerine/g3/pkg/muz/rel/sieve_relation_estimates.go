package rel

// Source: src/muz/rel/dl_sieve_relation.h
// sieve_relation::get_size_estimate_rows and get_size_estimate_bytes.

// SizeEstimateRows forwards the concrete inner-relation estimate. Ignored
// columns denote full domains but do not materialize rows in a sieve.
func (r *SieveRelation) SizeEstimateRows() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.inner.SizeEstimateRows()
}

// SizeEstimateBytes forwards the concrete inner payload estimate.
func (r *SieveRelation) SizeEstimateBytes() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.inner.SizeEstimateBytes()
}
