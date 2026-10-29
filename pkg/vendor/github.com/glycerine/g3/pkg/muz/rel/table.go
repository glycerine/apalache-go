package rel

// Source: src/muz/rel/dl_base.h/.cpp table_signature and table_base retained
// operations; src/muz/rel/dl_table.h/.cpp hashtable_table and its join.

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/glycerine/insdict"
)

var ErrTable = errors.New("muz/rel: invalid table operation")

type Element uint64
type Fact []Element

// Signature records the exclusive upper bound of each table column. The
// concrete hash table currently implements ordinary, non-functional columns.
type Signature struct {
	domains []uint64
}

func NewSignature(domains []uint64) (Signature, error) {
	result := Signature{domains: append([]uint64(nil), domains...)}
	for _, domain := range result.domains {
		if domain == 0 {
			return Signature{}, fmt.Errorf("%w: empty column domain", ErrTable)
		}
	}
	return result, nil
}

func (s Signature) Len() int { return len(s.domains) }

func (s Signature) Domain(column int) (uint64, error) {
	if column < 0 || column >= len(s.domains) {
		return 0, fmt.Errorf("%w: column index out of bounds", ErrTable)
	}
	return s.domains[column], nil
}

func (s Signature) Domains() []uint64 { return append([]uint64(nil), s.domains...) }

func (s Signature) equal(other Signature) bool {
	if len(s.domains) != len(other.domains) {
		return false
	}
	for i := range s.domains {
		if s.domains[i] != other.domains[i] {
			return false
		}
	}
	return true
}

func validateColumns(left, right Signature, leftColumns, rightColumns []int) error {
	if len(leftColumns) != len(rightColumns) {
		return fmt.Errorf("%w: join column count mismatch", ErrTable)
	}
	for i := range leftColumns {
		lc, rc := leftColumns[i], rightColumns[i]
		if lc < 0 || lc >= left.Len() || rc < 0 || rc >= right.Len() {
			return fmt.Errorf("%w: join column index out of bounds", ErrTable)
		}
		if left.domains[lc] != right.domains[rc] {
			return fmt.Errorf("%w: joined column domains differ", ErrTable)
		}
	}
	return nil
}

func projectedSignature(source Signature, removed []int) (Signature, error) {
	remove := make([]bool, source.Len())
	previous := -1
	for _, column := range removed {
		if column <= previous || column < 0 || column >= source.Len() {
			return Signature{}, fmt.Errorf("%w: projected columns must be strictly increasing and in range", ErrTable)
		}
		remove[column] = true
		previous = column
	}
	domains := make([]uint64, 0, source.Len()-len(removed))
	for i, domain := range source.domains {
		if !remove[i] {
			domains = append(domains, domain)
		}
	}
	return NewSignature(domains)
}

// HashTable is the concrete set-of-tuples table used by the retained Datalog
// relational path. Facts are copied on insertion and observation.
type HashTable struct {
	signature Signature
	rows      *insdict.Dict[string, Fact]
}

func NewHashTable(signature Signature) *HashTable {
	return &HashTable{signature: Signature{domains: signature.Domains()}, rows: insdict.NewDict[string, Fact]()}
}

func (t *HashTable) open() error {
	if t == nil || t.rows == nil {
		return ErrTable
	}
	return nil
}

func (t *HashTable) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: t.signature.Domains()}, nil
}

func (t *HashTable) validateFact(fact Fact) error {
	if len(fact) != t.signature.Len() {
		return fmt.Errorf("%w: fact arity mismatch", ErrTable)
	}
	for i, value := range fact {
		if uint64(value) >= t.signature.domains[i] {
			return fmt.Errorf("%w: fact value outside column domain", ErrTable)
		}
	}
	return nil
}

func factKey(fact Fact) string {
	bytes := make([]byte, 8*len(fact))
	for i, value := range fact {
		binary.LittleEndian.PutUint64(bytes[8*i:], uint64(value))
	}
	return string(bytes)
}

func cloneFact(fact Fact) Fact { return append(Fact(nil), fact...) }

// Add inserts a fact and reports whether it changed the table.
func (t *HashTable) Add(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if err := t.validateFact(fact); err != nil {
		return false, err
	}
	key := factKey(fact)
	if _, found := t.rows.Get2(key); found {
		return false, nil
	}
	t.rows.Set(key, cloneFact(fact))
	return true, nil
}

func (t *HashTable) Remove(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if err := t.validateFact(fact); err != nil {
		return false, err
	}
	key := factKey(fact)
	if _, found := t.rows.Get2(key); !found {
		return false, nil
	}
	t.rows.Del(key)
	return true, nil
}

func (t *HashTable) Contains(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if err := t.validateFact(fact); err != nil {
		return false, err
	}
	_, found := t.rows.Get2(factKey(fact))
	return found, nil
}

func (t *HashTable) Len() (int, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.rows.Len(), nil
}

func (t *HashTable) Facts() ([]Fact, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	result := make([]Fact, 0, t.rows.Len())
	for _, fact := range t.rows.All() {
		result = append(result, cloneFact(fact))
	}
	return result, nil
}

func (t *HashTable) Clone() (*HashTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	result := NewHashTable(t.signature)
	for key, fact := range t.rows.All() {
		result.rows.Set(key, cloneFact(fact))
	}
	return result, nil
}

func (t *HashTable) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	t.rows.Clear()
	return nil
}

// FilterEqual retains rows whose selected column equals value.
func (t *HashTable) FilterEqual(value Element, column int) error {
	if err := t.open(); err != nil {
		return err
	}
	if column < 0 || column >= t.signature.Len() || uint64(value) >= t.signature.domains[column] {
		return fmt.Errorf("%w: filter value or column out of range", ErrTable)
	}
	for key, fact := range t.rows.All() {
		if fact[column] != value {
			t.rows.Del(key)
		}
	}
	return nil
}

// FilterIdentical retains rows whose listed columns all have the same value.
func (t *HashTable) FilterIdentical(columns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	for i, column := range columns {
		if column < 0 || column >= t.signature.Len() {
			return fmt.Errorf("%w: identical-filter column out of range", ErrTable)
		}
		if i != 0 && t.signature.domains[column] != t.signature.domains[columns[0]] {
			return fmt.Errorf("%w: identical-filter column domains differ", ErrTable)
		}
	}
	if len(columns) < 2 {
		return nil
	}
	for key, fact := range t.rows.All() {
		value := fact[columns[0]]
		match := true
		for _, column := range columns[1:] {
			if fact[column] != value {
				match = false
				break
			}
		}
		if !match {
			t.rows.Del(key)
		}
	}
	return nil
}

// Join performs the retained nested-loop hashtable join. Result rows contain
// the complete left tuple followed by the complete right tuple.
func (t *HashTable) Join(other *HashTable, leftColumns, rightColumns []int) (*HashTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if err := other.open(); err != nil {
		return nil, err
	}
	if err := validateColumns(t.signature, other.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	domains := append(t.signature.Domains(), other.signature.domains...)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	result := NewHashTable(signature)
	for _, left := range t.rows.All() {
		for _, right := range other.rows.All() {
			matches := true
			for i := range leftColumns {
				if left[leftColumns[i]] != right[rightColumns[i]] {
					matches = false
					break
				}
			}
			if matches {
				fact := append(cloneFact(left), right...)
				if _, err = result.Add(fact); err != nil {
					return nil, err
				}
			}
		}
	}
	return result, nil
}

func (t *HashTable) Project(removed []int) (*HashTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	signature, err := projectedSignature(t.signature, removed)
	if err != nil {
		return nil, err
	}
	result := NewHashTable(signature)
	remove := make([]bool, t.signature.Len())
	for _, column := range removed {
		remove[column] = true
	}
	for _, source := range t.rows.All() {
		fact := make(Fact, 0, signature.Len())
		for i, value := range source {
			if !remove[i] {
				fact = append(fact, value)
			}
		}
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (t *HashTable) SelectEqualAndProject(value Element, column int) (*HashTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if column < 0 || column >= t.signature.Len() || uint64(value) >= t.signature.domains[column] {
		return nil, fmt.Errorf("%w: selection value or column out of range", ErrTable)
	}
	signature, err := projectedSignature(t.signature, []int{column})
	if err != nil {
		return nil, err
	}
	result := NewHashTable(signature)
	for _, source := range t.rows.All() {
		if source[column] != value {
			continue
		}
		fact := make(Fact, 0, signature.Len())
		fact = append(fact, source[:column]...)
		fact = append(fact, source[column+1:]...)
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Permute renames columns using result[i] = source[permutation[i]].
func (t *HashTable) Permute(permutation []int) (*HashTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if len(permutation) != t.signature.Len() {
		return nil, fmt.Errorf("%w: permutation arity mismatch", ErrTable)
	}
	seen := make([]bool, len(permutation))
	domains := make([]uint64, len(permutation))
	for i, source := range permutation {
		if source < 0 || source >= len(permutation) || seen[source] {
			return nil, fmt.Errorf("%w: invalid column permutation", ErrTable)
		}
		seen[source] = true
		domains[i] = t.signature.domains[source]
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	result := NewHashTable(signature)
	for _, source := range t.rows.All() {
		fact := make(Fact, len(permutation))
		for i, column := range permutation {
			fact[i] = source[column]
		}
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// RenameCycle applies the source cycle direction: each listed column receives
// the next column's value and the last receives the first.
func (t *HashTable) RenameCycle(cycle []int) (*HashTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if len(cycle) < 2 {
		return t.Clone()
	}
	permutation := make([]int, t.signature.Len())
	for i := range permutation {
		permutation[i] = i
	}
	seen := make(map[int]bool)
	for _, column := range cycle {
		if column < 0 || column >= len(permutation) || seen[column] {
			return nil, fmt.Errorf("%w: invalid rename cycle", ErrTable)
		}
		seen[column] = true
	}
	for i := 1; i < len(cycle); i++ {
		permutation[cycle[i-1]] = cycle[i]
	}
	permutation[cycle[len(cycle)-1]] = cycle[0]
	return t.Permute(permutation)
}

// Union inserts all source rows and optionally records only newly inserted
// rows in delta.
func (t *HashTable) Union(source, delta *HashTable) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	if !t.signature.equal(source.signature) {
		return fmt.Errorf("%w: union signatures differ", ErrTable)
	}
	if delta != nil {
		if err := delta.open(); err != nil {
			return err
		}
		if !t.signature.equal(delta.signature) {
			return fmt.Errorf("%w: delta signature differs", ErrTable)
		}
	}
	for _, fact := range source.rows.All() {
		inserted, err := t.Add(fact)
		if err != nil {
			return err
		}
		if inserted && delta != nil {
			if _, err = delta.Add(fact); err != nil {
				return err
			}
		}
	}
	return nil
}

// FilterByNegation removes target rows having a matching row in negated.
func (t *HashTable) FilterByNegation(negated *HashTable, targetColumns, negatedColumns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := negated.open(); err != nil {
		return err
	}
	if err := validateColumns(t.signature, negated.signature, targetColumns, negatedColumns); err != nil {
		return err
	}
	if len(targetColumns) == 0 {
		if negated.rows.Len() != 0 {
			return t.Reset()
		}
		return nil
	}
	for key, target := range t.rows.All() {
		remove := false
		for _, other := range negated.rows.All() {
			matches := true
			for i := range targetColumns {
				if target[targetColumns[i]] != other[negatedColumns[i]] {
					matches = false
					break
				}
			}
			if matches {
				remove = true
				break
			}
		}
		if remove {
			t.rows.Del(key)
		}
	}
	return nil
}
