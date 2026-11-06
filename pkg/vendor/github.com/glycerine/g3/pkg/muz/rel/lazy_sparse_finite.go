package rel

import (
	"errors"
	"io"
)

func (e *LazySparseEngine) adoptSparse(table *SparseTable) (*LazySparseTable, error) {
	if err := e.open(); err != nil || table == nil || table.open() != nil {
		return nil, ErrLazySparse
	}
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	functional, err := table.FunctionalColumns()
	if err != nil {
		return nil, err
	}
	return e.adopt(&lazySparseNode{kind: LazyTableBase, signature: signature, functional: functional, evaluated: table}), nil
}

func (t *LazySparseTable) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: t.root.signature.Domains()}, nil
}

func (t *LazySparseTable) Contains(fact Fact) (bool, error) {
	table, err := t.finiteEval()
	if err != nil {
		return false, err
	}
	return table.Contains(fact)
}

func (t *LazySparseTable) Len() (int, error) {
	table, err := t.finiteEval()
	if err != nil {
		return 0, err
	}
	return table.Len()
}

// FetchFact forces this lazy root and reads the functional suffix from its
// actual packed sparse owner.
func (t *LazySparseTable) FetchFact(fact Fact) (bool, error) {
	table, err := t.eval()
	if err != nil {
		return false, err
	}
	return table.FetchFact(fact)
}

// EnsureFact forces this lazy root and mutates its actual packed sparse owner.
func (t *LazySparseTable) EnsureFact(fact Fact) error {
	table, err := t.eval()
	if err != nil {
		return err
	}
	return table.EnsureFact(fact)
}

// Remove forces this lazy root and removes the fact from its actual packed
// sparse owner or the source nullary Hash fallback.
func (t *LazySparseTable) Remove(fact Fact) (bool, error) {
	table, err := t.finiteEval()
	if err != nil {
		return false, err
	}
	switch concrete := table.(type) {
	case *SparseTable:
		return concrete.Remove(fact)
	case *HashTable:
		return concrete.Remove(fact)
	default:
		return false, ErrLazySparse
	}
}

func (t *LazySparseTable) SizeEstimateRows() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return 1, nil
}

func (t *LazySparseTable) SizeEstimateBytes() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return 1, nil
}

func (t *LazySparseTable) Display(out io.Writer) error {
	table, err := t.finiteEval()
	if err != nil {
		return err
	}
	return displayLazySparseFinite(table, out)
}

func (t *LazySparseTable) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.root.signature.Len() == 0 {
		signature, _ := NewSignature(nil)
		next := &lazySparseNode{kind: LazyTableBase, references: 1, signature: signature, nullary: NewHashTable(signature)}
		if err := releaseLazySparseNode(t.root); err != nil {
			return errors.Join(err, next.nullary.Close())
		}
		t.root = next
		return nil
	}
	table, err := NewSparseTable(t.root.signature, t.root.functional)
	if err != nil {
		return err
	}
	next := &lazySparseNode{kind: LazyTableBase, references: 1, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional, evaluated: table}
	if err = releaseLazySparseNode(t.root); err != nil {
		return errors.Join(err, table.Close())
	}
	t.root = next
	return nil
}

func (t *LazySparseTable) CloneFinite() (FiniteTable, error) {
	return t.Clone()
}

var _ FiniteTable = (*LazySparseTable)(nil)
