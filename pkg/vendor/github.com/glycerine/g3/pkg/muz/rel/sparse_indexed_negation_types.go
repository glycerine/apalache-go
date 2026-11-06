package rel

// Source: src/muz/rel/dl_sparse_table.cpp
// sparse_table_plugin::negation_filter_fn.

type SparseIndexedNegationPlan struct {
	targetSignature    Signature
	negativeSignature  Signature
	targetFunctional   int
	negativeFunctional int
	targetColumns      []int
	negativeColumns    []int
}

type SparseIndexedNegationStats struct {
	TargetRows, NegativeRows int
	Probes, IndexGroups      int
	Candidates, Removed      int
	UsedNegativeOuter        bool
	ClearedProduct           bool
}
