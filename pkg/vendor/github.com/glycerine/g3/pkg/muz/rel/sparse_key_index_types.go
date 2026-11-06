package rel

// Source: src/muz/rel/dl_sparse_table.cpp sparse_table::key_indexer,
// general_key_indexer, full_signature_key_indexer and get_key_indexer.

type SparseKeyLookupPlan struct {
	signature  Signature
	functional int
	columns    []int
}

type SparseKeyGroup struct {
	Key  Fact
	Rows []Fact
}

type SparseKeyIndexSnapshot struct {
	plan   *SparseKeyLookupPlan
	groups []SparseKeyGroup
	byKey  map[string]int
}

type SparseKeyIndexStats struct {
	Rows          int
	Groups        int
	KeyColumns    int
	FullSignature bool
}
