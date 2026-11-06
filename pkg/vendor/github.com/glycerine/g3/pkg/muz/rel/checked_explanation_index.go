package rel

// Source: dl_check_table.cpp paired table operations and
// dl_finite_product_relation.cpp's one functional inner-relation index.

import (
	"errors"
	"fmt"
	"io"
)

// ExplanationIndexTable is the concrete outer table used by the live
// fact-explanation finite-product instantiation. Its final column is the one
// functional inner-explanation index.
type ExplanationIndexTable interface {
	open() error
	Signature() (Signature, error)
	Facts() ([]Fact, error)
	FetchFact(Fact) (bool, error)
	EnsureFact(Fact) error
	Remove(Fact) (bool, error)
	Len() (int, error)
	SizeEstimateRows() (uint32, error)
	SizeEstimateBytes() (uint32, error)
	Display(io.Writer) error
	Close() error
}

var _ ExplanationIndexTable = (*SparseTable)(nil)

// CheckedExplanationIndex is check_table specialized to the functional outer
// index of a fact-level explanation product. Both configured concrete tables
// are independently updated and every observable row/index result is checked.
type CheckedExplanationIndex struct {
	primary    FiniteTable
	checker    FiniteTable
	functional int
	closed     bool
}

var _ ExplanationIndexTable = (*CheckedExplanationIndex)(nil)

func newCheckedExplanationIndex(factory *FiniteCheckedFactory, signature Signature, functional int) (_ *CheckedExplanationIndex, resultErr error) {
	if factory == nil || functional != 1 || signature.Len() < functional || !factory.CanHandleSignature(signature) {
		return nil, ErrCheckTable
	}
	primary, err := factory.table(factory.primaryKind, signature, functional)
	if err != nil {
		return nil, err
	}
	checker, err := factory.table(factory.checkerKind, signature, functional)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptCheckedExplanationIndex(primary, checker, functional)
}

func adoptCheckedExplanationIndex(primary, checker FiniteTable, functional int) (_ *CheckedExplanationIndex, resultErr error) {
	result := &CheckedExplanationIndex{primary: primary, checker: checker, functional: functional}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err := result.verify("construction"); err != nil {
		return nil, err
	}
	return result, nil
}

func newExplanationIndexLike(template FiniteTable, signature Signature, functional int) (FiniteTable, error) {
	if functional != 1 || signature.Len() < functional {
		return nil, ErrCheckTable
	}
	switch concrete := template.(type) {
	case *HashTable:
		return NewHashTable(signature), nil
	case *SparseTable:
		return NewSparseTable(signature, functional)
	case *LazySparseTable:
		if err := concrete.open(); err != nil {
			return nil, err
		}
		return concrete.engine.New(signature, functional)
	default:
		return nil, ErrCheckTable
	}
}

// NewLike creates an empty checked functional index using the same two
// concrete plugins. Lazy sides borrow their existing context engine.
func (t *CheckedExplanationIndex) NewLike(signature Signature, functional int) (_ *CheckedExplanationIndex, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	primary, err := newExplanationIndexLike(t.primary, signature, functional)
	if err != nil {
		return nil, err
	}
	checker, err := newExplanationIndexLike(t.checker, signature, functional)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptCheckedExplanationIndex(primary, checker, functional)
}

// NewExplanationIndex constructs a direct reusable Go checker pair. Source
// favourite selection additionally applies CanHandleExplanationIndex in
// RelationManager.NewExplanationIndexTable before calling this helper.
func (f *FiniteCheckedFactory) NewExplanationIndex(signature Signature, functional int) (*CheckedExplanationIndex, error) {
	if f == nil {
		return nil, ErrCheckTable
	}
	return newCheckedExplanationIndex(f, signature, functional)
}

// NewExplanationIndexTable follows the manager's current favourite table,
// including a borrowed checked factory installed for one explanation query.
func (m *RelationManager) NewExplanationIndexTable(signature Signature, functional int) (ExplanationIndexTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if functional != 1 || signature.Len() < functional {
		return nil, ErrTable
	}
	if m.checkedFinite != nil {
		if m.checkedFinite.CanHandleExplanationIndex(signature, functional) {
			return m.checkedFinite.NewExplanationIndex(signature, functional)
		}
		return NewSparseTable(signature, functional)
	}
	switch m.defaultFinite {
	case "lazy_sparse":
		if m.lazyFinite == nil {
			return nil, ErrTable
		}
		return m.lazyFinite.NewFunctional(signature, functional)
	default:
		// Hash rejects functional columns, so source registration order falls
		// through to Sparse, the first plugin that accepts this signature.
		return NewSparseTable(signature, functional)
	}
}

func (t *CheckedExplanationIndex) open() error {
	if t == nil || t.closed || t.primary == nil || t.checker == nil || t.functional != 1 {
		return ErrCheckTable
	}
	return nil
}

func (t *CheckedExplanationIndex) verify(objective string) error {
	if err := t.open(); err != nil {
		return err
	}
	a, err := t.primary.Facts()
	if err != nil {
		return err
	}
	b, err := t.checker.Facts()
	if err != nil {
		return err
	}
	if len(a) != len(b) {
		return fmt.Errorf("%w: explanation index %s row counts differ", ErrCheckTable, objective)
	}
	containsExact := func(rows []Fact, wanted Fact) bool {
		for _, row := range rows {
			if len(row) != len(wanted) {
				continue
			}
			equal := true
			for column := range row {
				if row[column] != wanted[column] {
					equal = false
					break
				}
			}
			if equal {
				return true
			}
		}
		return false
	}
	for _, row := range a {
		if !containsExact(b, row) {
			return fmt.Errorf("%w: explanation index %s checker misses %v", ErrCheckTable, objective, row)
		}
	}
	for _, row := range b {
		if !containsExact(a, row) {
			return fmt.Errorf("%w: explanation index %s primary misses %v", ErrCheckTable, objective, row)
		}
	}
	return nil
}

func (t *CheckedExplanationIndex) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return t.primary.Signature()
}

func (t *CheckedExplanationIndex) Facts() ([]Fact, error) {
	if err := t.verify("facts"); err != nil {
		return nil, err
	}
	return t.primary.Facts()
}

func explanationIndexFetch(table FiniteTable, fact Fact, functional int) (bool, error) {
	switch concrete := table.(type) {
	case *SparseTable:
		return concrete.FetchFact(fact)
	case *LazySparseTable:
		return concrete.FetchFact(fact)
	}
	signature, err := table.Signature()
	if err != nil || len(fact) != signature.Len() || functional <= 0 || functional > signature.Len() {
		return false, errors.Join(err, ErrCheckTable)
	}
	rows, err := table.Facts()
	if err != nil {
		return false, err
	}
	boundary := signature.Len() - functional
	var match Fact
	for _, row := range rows {
		equal := true
		for column := 0; column < boundary; column++ {
			if row[column] != fact[column] {
				equal = false
				break
			}
		}
		if !equal {
			continue
		}
		if match != nil {
			return false, fmt.Errorf("%w: duplicate functional explanation key", ErrCheckTable)
		}
		match = row
	}
	if match == nil {
		return false, nil
	}
	copy(fact[boundary:], match[boundary:])
	return true, nil
}

func (t *CheckedExplanationIndex) FetchFact(fact Fact) (bool, error) {
	if err := t.verify("fetch"); err != nil {
		return false, err
	}
	if err := validateExplanationIndexFact(t.primary, fact); err != nil {
		return false, err
	}
	a, b := append(Fact(nil), fact...), append(Fact(nil), fact...)
	foundA, err := explanationIndexFetch(t.primary, a, t.functional)
	if err != nil {
		return false, err
	}
	foundB, err := explanationIndexFetch(t.checker, b, t.functional)
	if err != nil {
		return false, err
	}
	if foundA != foundB {
		return false, fmt.Errorf("%w: explanation index fetch results differ", ErrCheckTable)
	}
	if !foundA {
		return false, nil
	}
	boundary := len(fact) - t.functional
	for column := boundary; column < len(fact); column++ {
		if a[column] != b[column] {
			return false, fmt.Errorf("%w: explanation index functional values differ", ErrCheckTable)
		}
	}
	copy(fact[boundary:], a[boundary:])
	return true, nil
}

func validateExplanationIndexFact(table FiniteTable, fact Fact) error {
	signature, err := table.Signature()
	if err != nil {
		return err
	}
	if len(fact) != signature.Len() {
		return ErrCheckTable
	}
	for column, value := range fact {
		domain, err := signature.Domain(column)
		if err != nil {
			return err
		}
		if uint64(value) >= domain {
			return ErrCheckTable
		}
	}
	return nil
}

func explanationIndexEnsure(table FiniteTable, fact Fact, functional int) error {
	switch concrete := table.(type) {
	case *SparseTable:
		return concrete.EnsureFact(fact)
	case *LazySparseTable:
		return concrete.EnsureFact(fact)
	}
	current := append(Fact(nil), fact...)
	found, err := explanationIndexFetch(table, current, functional)
	if err != nil {
		return err
	}
	if found {
		same := true
		for i := range fact {
			if current[i] != fact[i] {
				same = false
				break
			}
		}
		if same {
			return nil
		}
		switch concrete := table.(type) {
		case *HashTable:
			if _, err = concrete.Remove(current); err != nil {
				return err
			}
		default:
			return ErrCheckTable
		}
	}
	_, err = table.Add(fact)
	return err
}

func (t *CheckedExplanationIndex) EnsureFact(fact Fact) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := validateExplanationIndexFact(t.primary, fact); err != nil {
		return err
	}
	if err := explanationIndexEnsure(t.primary, append(Fact(nil), fact...), t.functional); err != nil {
		return err
	}
	if err := explanationIndexEnsure(t.checker, append(Fact(nil), fact...), t.functional); err != nil {
		return err
	}
	return t.verify("ensure")
}

func explanationIndexRemove(table FiniteTable, fact Fact) (bool, error) {
	switch concrete := table.(type) {
	case *HashTable:
		return concrete.Remove(fact)
	case *SparseTable:
		return concrete.Remove(fact)
	case *LazySparseTable:
		return concrete.Remove(fact)
	default:
		return false, ErrCheckTable
	}
}

func (t *CheckedExplanationIndex) Remove(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if err := validateExplanationIndexFact(t.primary, fact); err != nil {
		return false, err
	}
	presentPrimary, err := t.primary.Contains(fact)
	if err != nil {
		return false, err
	}
	presentChecker, err := t.checker.Contains(fact)
	if err != nil {
		return false, err
	}
	if presentPrimary != presentChecker {
		return false, fmt.Errorf("%w: explanation index removal membership differs", ErrCheckTable)
	}
	if !presentPrimary {
		return false, nil
	}
	a, err := explanationIndexRemove(t.primary, fact)
	if err != nil {
		return false, err
	}
	b, err := explanationIndexRemove(t.checker, fact)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, fmt.Errorf("%w: explanation index removal results differ", ErrCheckTable)
	}
	return a, t.verify("remove")
}

func (t *CheckedExplanationIndex) Len() (int, error) {
	if err := t.verify("length"); err != nil {
		return 0, err
	}
	return t.primary.Len()
}

func explanationIndexSizeRows(table FiniteTable) (uint32, error) {
	switch concrete := table.(type) {
	case *HashTable:
		return concrete.SizeEstimateRows()
	case *SparseTable:
		return concrete.SizeEstimateRows()
	case *LazySparseTable:
		return concrete.SizeEstimateRows()
	default:
		return 0, ErrCheckTable
	}
}

func explanationIndexSizeBytes(table FiniteTable) (uint32, error) {
	switch concrete := table.(type) {
	case *HashTable:
		return concrete.SizeEstimateBytes()
	case *SparseTable:
		return concrete.SizeEstimateBytes()
	case *LazySparseTable:
		return concrete.SizeEstimateBytes()
	default:
		return 0, ErrCheckTable
	}
}

func (t *CheckedExplanationIndex) SizeEstimateRows() (uint32, error) {
	if err := t.verify("size rows"); err != nil {
		return 0, err
	}
	return explanationIndexSizeRows(t.primary)
}

func (t *CheckedExplanationIndex) SizeEstimateBytes() (uint32, error) {
	if err := t.verify("size bytes"); err != nil {
		return 0, err
	}
	return explanationIndexSizeBytes(t.primary)
}

func (t *CheckedExplanationIndex) Display(out io.Writer) error {
	if err := t.verify("display"); err != nil {
		return err
	}
	switch concrete := t.primary.(type) {
	case *HashTable:
		return concrete.Display(out)
	case *SparseTable:
		return concrete.Display(out)
	case *LazySparseTable:
		return concrete.Display(out)
	default:
		return ErrCheckTable
	}
}

func (t *CheckedExplanationIndex) ConcreteKinds() (primary, checker string, err error) {
	if err = t.open(); err != nil {
		return "", "", err
	}
	kind := func(table FiniteTable) string {
		switch table.(type) {
		case *HashTable:
			return "hashtable"
		case *SparseTable:
			return "sparse"
		case *LazySparseTable:
			return "lazy_sparse"
		default:
			return fmt.Sprintf("%T", table)
		}
	}
	return kind(t.primary), kind(t.checker), nil
}

func (t *CheckedExplanationIndex) Close() (result error) {
	if t == nil || t.closed {
		return nil
	}
	if t.primary != nil {
		if err := t.primary.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			t.primary = nil
		}
	}
	if t.checker != nil {
		if err := t.checker.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			t.checker = nil
		}
	}
	if result == nil {
		t.closed = true
	}
	return result
}
