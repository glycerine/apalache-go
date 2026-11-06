package rel

// Source: src/muz/rel/dl_base.h table_signature and dl_base.cpp
// table_signature::{from_join,from_project,from_join_project}.

func (table *SparseTable) FunctionalColumns() (int, error) {
	if err := table.open(); err != nil {
		return 0, err
	}
	return table.functional, nil
}

func (table *SparseTable) FirstFunctionalColumn() (int, error) {
	if err := table.open(); err != nil {
		return 0, err
	}
	return table.signature.Len() - table.functional, nil
}

func sparseFunctionalSignature(table *SparseTable) (Signature, int, error) {
	if err := table.open(); err != nil {
		return Signature{}, 0, err
	}
	return Signature{domains: table.signature.Domains()}, table.functional, nil
}
