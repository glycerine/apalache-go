package rel

// Source: src/muz/rel/dl_sparse_table.cpp
// sparse_table_plugin::select_equal_and_project_fn.

type SparseIndexedSelectPlan struct {
	signature        Signature
	functional       int
	column           int
	value            Element
	resultSignature  Signature
	resultFunctional int
}

type SparseIndexedSelectStats struct {
	Rows, IndexGroups int
	Matches, Inserted int
	FullSignature     bool
}
