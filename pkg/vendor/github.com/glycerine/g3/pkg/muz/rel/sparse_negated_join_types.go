package rel

// Source: src/muz/rel/dl_sparse_table.cpp
// sparse_table_plugin::negated_join_fn and mk_filter_by_negated_join_fn.

type SparseNegatedJoinPlan struct {
	targetSignature   Signature
	firstSignature    Signature
	secondSignature   Signature
	targetFunctional  int
	firstFunctional   int
	secondFunctional  int
	targetColumns     []int
	sourceColumns     []int
	firstJoinColumns  []int
	secondJoinColumns []int
}

type SparseNegatedJoinStats struct {
	TargetRows   int
	FirstRows    int
	SecondRows   int
	FirstProbes  int
	SecondProbes int
	Removed      int
}
