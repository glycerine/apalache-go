package rel

// Source: src/muz/rel/dl_sparse_table.cpp join_project_fn and
// sparse_table::self_agnostic_join_project.

type SparseIndexedJoinPlan struct {
	leftSignature    Signature
	rightSignature   Signature
	leftFunctional   int
	rightFunctional  int
	leftColumns      []int
	rightColumns     []int
	removed          []int
	resultSignature  Signature
	resultFunctional int
}

type SparseIndexedJoinStats struct {
	LeftRows, RightRows     int
	Probes, IndexGroups     int
	CandidateRows, Inserted int
	IndexedLeft, Product    bool
}
