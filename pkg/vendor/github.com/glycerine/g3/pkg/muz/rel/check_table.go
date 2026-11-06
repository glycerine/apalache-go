package rel

// Source: src/muz/rel/dl_check_table.h/.cpp check_table storage and direct
// operations. This concrete specialization runs two independently owned
// HashTables in lockstep and checks exact row equivalence.

import (
	"errors"
	"fmt"
)

var ErrCheckTable = errors.New("muz/rel: checked table mismatch")

type CheckTable struct {
	signature Signature
	primary   *HashTable
	checker   *HashTable
	checks    uint64
	closed    bool
}

func NewCheckTable(signature Signature) (*CheckTable, error) {
	return AdoptCheckTable(NewHashTable(signature), NewHashTable(signature))
}

// AdoptCheckTable transfers both tables to the result and validates their
// signatures and rows before publication.
func AdoptCheckTable(primary, checker *HashTable) (*CheckTable, error) {
	if primary == nil || checker == nil || primary.open() != nil || checker.open() != nil || primary == checker {
		return nil, ErrCheckTable
	}
	primarySignature, err := primary.Signature()
	if err != nil {
		return nil, err
	}
	checkerSignature, err := checker.Signature()
	if err != nil {
		return nil, err
	}
	if !primarySignature.equal(checkerSignature) {
		return nil, fmt.Errorf("%w: signatures differ", ErrCheckTable)
	}
	table := &CheckTable{signature: primarySignature, primary: primary, checker: checker}
	if err = table.Validate(); err != nil {
		return nil, err
	}
	return table, nil
}

func (t *CheckTable) open() error {
	if t == nil || t.closed || t.primary == nil || t.checker == nil || t.primary.open() != nil || t.checker.open() != nil {
		return ErrCheckTable
	}
	return nil
}

func (t *CheckTable) Close() error {
	if t == nil || t.closed {
		return nil
	}
	t.closed = true
	primary, checker := t.primary, t.checker
	t.primary = nil
	t.checker = nil
	return errors.Join(primary.Close(), checker.Close())
}

func (t *CheckTable) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: t.signature.Domains()}, nil
}

func (t *CheckTable) CheckCount() uint64 {
	if t == nil {
		return 0
	}
	return t.checks
}

// Validate performs the source two-way row containment check.
func (t *CheckTable) Validate() error {
	if err := t.open(); err != nil {
		return err
	}
	t.checks++
	primaryFacts, err := t.primary.Facts()
	if err != nil {
		return err
	}
	checkerFacts, err := t.checker.Facts()
	if err != nil {
		return err
	}
	if len(primaryFacts) != len(checkerFacts) {
		return fmt.Errorf("%w at check %d: row counts %d and %d", ErrCheckTable, t.checks, len(primaryFacts), len(checkerFacts))
	}
	for _, fact := range primaryFacts {
		found, containsErr := t.checker.Contains(fact)
		if containsErr != nil {
			return containsErr
		}
		if !found {
			return fmt.Errorf("%w at check %d: checker misses %v", ErrCheckTable, t.checks, fact)
		}
	}
	for _, fact := range checkerFacts {
		found, containsErr := t.primary.Contains(fact)
		if containsErr != nil {
			return containsErr
		}
		if !found {
			return fmt.Errorf("%w at check %d: primary misses %v", ErrCheckTable, t.checks, fact)
		}
	}
	return nil
}

// Primary returns the borrowed implementation under test after validation.
func (t *CheckTable) Primary() (*HashTable, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t.primary, nil
}

func (t *CheckTable) Empty() (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	primaryLength, err := t.primary.Len()
	if err != nil {
		return false, err
	}
	checkerLength, err := t.checker.Len()
	if err != nil {
		return false, err
	}
	if (primaryLength == 0) != (checkerLength == 0) {
		return false, fmt.Errorf("%w: empty results differ", ErrCheckTable)
	}
	return primaryLength == 0, nil
}

// SizeEstimateRows forwards the estimate from the implementation under test,
// matching check_table::get_size_estimate_rows.
func (t *CheckTable) SizeEstimateRows() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.primary.SizeEstimateRows()
}

// SizeEstimateBytes forwards the estimate from the implementation under test,
// matching check_table::get_size_estimate_bytes.
func (t *CheckTable) SizeEstimateBytes() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.primary.SizeEstimateBytes()
}

func (t *CheckTable) Contains(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	// Source uses the independent checker for membership queries.
	return t.checker.Contains(fact)
}

func (t *CheckTable) Add(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	primaryInserted, err := t.primary.Add(fact)
	if err != nil {
		return false, err
	}
	checkerInserted, err := t.checker.Add(fact)
	if err != nil {
		return false, err
	}
	if primaryInserted != checkerInserted {
		return false, fmt.Errorf("%w: insertion results differ", ErrCheckTable)
	}
	return primaryInserted, t.Validate()
}

func (t *CheckTable) Remove(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	primaryRemoved, err := t.primary.Remove(fact)
	if err != nil {
		return false, err
	}
	checkerRemoved, err := t.checker.Remove(fact)
	if err != nil {
		return false, err
	}
	if primaryRemoved != checkerRemoved {
		return false, fmt.Errorf("%w: removal results differ", ErrCheckTable)
	}
	return primaryRemoved, t.Validate()
}

func (t *CheckTable) Reset() error {
	facts, err := t.Facts()
	if err != nil {
		return err
	}
	return t.RemoveFacts(facts)
}

func (t *CheckTable) Facts() ([]Fact, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t.primary.Facts()
}

func (t *CheckTable) Clone() (*CheckTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	primary, err := t.primary.Clone()
	if err != nil {
		return nil, err
	}
	checker, err := t.checker.Clone()
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptCheckedTables(primary, checker)
}

func complementCheckedHashTable(source *HashTable) (*HashTable, error) {
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	if signature.Len() > 1 {
		return nil, fmt.Errorf("%w: complement requires compiler decomposition", ErrCheckTable)
	}
	result := NewHashTable(signature)
	if signature.Len() == 0 {
		length, lengthErr := source.Len()
		if lengthErr != nil {
			return nil, lengthErr
		}
		if length == 0 {
			_, err = result.Add(Fact{})
		}
		return result, err
	}
	for value := uint64(0); value < signature.domains[0]; value++ {
		fact := Fact{Element(value)}
		found, containsErr := source.Contains(fact)
		if containsErr != nil {
			return nil, containsErr
		}
		if !found {
			if _, err = result.Add(fact); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func (t *CheckTable) Complement() (*CheckTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	primary, err := complementCheckedHashTable(t.primary)
	if err != nil {
		return nil, err
	}
	checker, err := complementCheckedHashTable(t.checker)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptCheckedTables(primary, checker)
}
