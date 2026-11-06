package rel

// FiniteTable is the live finite-storage portion of source table_base. The
// implementations own their concrete row storage; no representation aliases
// or conversions occur through this boundary.
type FiniteTable interface {
	Signature() (Signature, error)
	Facts() ([]Fact, error)
	Add(Fact) (bool, error)
	Contains(Fact) (bool, error)
	Len() (int, error)
	Reset() error
	Close() error
	CloneFinite() (FiniteTable, error)
}

func (t *HashTable) CloneFinite() (FiniteTable, error) {
	clone, err := t.Clone()
	if err != nil {
		return nil, err
	}
	return clone, nil
}
func (t *SparseTable) CloneFinite() (FiniteTable, error) {
	clone, err := t.Clone()
	if err != nil {
		return nil, err
	}
	return clone, nil
}
func (t *SparseTable) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: t.signature.Domains()}, nil
}

var _ FiniteTable = (*HashTable)(nil)
var _ FiniteTable = (*SparseTable)(nil)
