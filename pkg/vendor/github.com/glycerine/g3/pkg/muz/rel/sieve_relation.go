package rel

// Source: src/muz/rel/dl_sieve_relation.h and the sieve_relation storage,
// fact, clone, complement, and display operations in dl_sieve_relation.cpp.
// This is the retained concrete HashTable specialization. The source plugin
// registry remains with the relation-manager plugin integration layer.

import (
	"errors"
	"fmt"
	"strings"
)

var ErrSieve = errors.New("muz/rel: invalid sieve relation operation")

// SieveRelation stores only selected columns in its inner table. Unselected
// columns denote their full domains and are deliberately ignored by facts and
// operations, matching the source relation's over-approximation.
type SieveRelation struct {
	signature  Signature
	innerCols  []bool
	sigToInner []int
	innerToSig []int
	ignored    []int
	inner      *HashTable
	closed     bool
}

func NewSieveRelation(signature Signature, innerColumns []bool) (*SieveRelation, error) {
	domains := signature.Domains()
	if len(innerColumns) != len(domains) {
		return nil, fmt.Errorf("%w: inner-column mask arity mismatch", ErrSieve)
	}
	innerDomains := make([]uint64, 0, len(domains))
	for column, selected := range innerColumns {
		if selected {
			innerDomains = append(innerDomains, domains[column])
		}
	}
	innerSignature, err := NewSignature(innerDomains)
	if err != nil {
		return nil, err
	}
	return AdoptSieveRelation(signature, innerColumns, NewHashTable(innerSignature))
}

// AdoptSieveRelation transfers ownership of inner to the returned relation.
func AdoptSieveRelation(signature Signature, innerColumns []bool, inner *HashTable) (*SieveRelation, error) {
	if inner == nil || inner.open() != nil || len(innerColumns) != signature.Len() {
		return nil, ErrSieve
	}
	innerSignature, err := inner.Signature()
	if err != nil {
		return nil, err
	}
	r := &SieveRelation{
		signature:  Signature{domains: signature.Domains()},
		innerCols:  append([]bool(nil), innerColumns...),
		sigToInner: make([]int, signature.Len()),
		inner:      inner,
	}
	for i := range r.sigToInner {
		r.sigToInner[i] = -1
	}
	for column, selected := range r.innerCols {
		if selected {
			innerColumn := len(r.innerToSig)
			if innerColumn >= innerSignature.Len() || innerSignature.domains[innerColumn] != r.signature.domains[column] {
				return nil, fmt.Errorf("%w: inner signature does not match selected columns", ErrSieve)
			}
			r.sigToInner[column] = innerColumn
			r.innerToSig = append(r.innerToSig, column)
		} else {
			r.ignored = append(r.ignored, column)
		}
	}
	if len(r.innerToSig) != innerSignature.Len() {
		return nil, fmt.Errorf("%w: inner signature arity mismatch", ErrSieve)
	}
	return r, nil
}

// NewFullSieve constructs the source representation of a full relation: all
// columns are ignored and the zero-column inner table contains its sole fact.
func NewFullSieve(signature Signature) (*SieveRelation, error) {
	r, err := NewSieveRelation(signature, make([]bool, signature.Len()))
	if err != nil {
		return nil, err
	}
	if _, err = r.inner.Add(Fact{}); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *SieveRelation) open() error {
	if r == nil || r.closed || r.inner == nil || r.inner.open() != nil {
		return ErrSieve
	}
	return nil
}

func (r *SieveRelation) Close() error {
	if r == nil || r.closed {
		return nil
	}
	if err := r.inner.Close(); err != nil {
		return err
	}
	r.closed = true
	r.inner = nil
	r.innerCols = nil
	r.sigToInner = nil
	r.innerToSig = nil
	r.ignored = nil
	return nil
}

func (r *SieveRelation) Signature() (Signature, error) {
	if err := r.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: r.signature.Domains()}, nil
}

func (r *SieveRelation) InnerSignature() (Signature, error) {
	if err := r.open(); err != nil {
		return Signature{}, err
	}
	return r.inner.Signature()
}

func (r *SieveRelation) InnerColumns() []bool {
	if r.open() != nil {
		return nil
	}
	return append([]bool(nil), r.innerCols...)
}

func (r *SieveRelation) IsInnerColumn(column int) bool {
	return r.open() == nil && column >= 0 && column < len(r.sigToInner) && r.sigToInner[column] >= 0
}

func (r *SieveRelation) InnerColumn(column int) (int, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	if column < 0 || column >= len(r.sigToInner) || r.sigToInner[column] < 0 {
		return 0, fmt.Errorf("%w: column is not retained", ErrSieve)
	}
	return r.sigToInner[column], nil
}

func (r *SieveRelation) NoSievedColumns() bool { return r.open() == nil && len(r.ignored) == 0 }
func (r *SieveRelation) NoInnerColumns() bool  { return r.open() == nil && len(r.innerToSig) == 0 }

func (r *SieveRelation) validateFact(fact Fact) error {
	if len(fact) != r.signature.Len() {
		return fmt.Errorf("%w: fact arity mismatch", ErrSieve)
	}
	for i, value := range fact {
		if uint64(value) >= r.signature.domains[i] {
			return fmt.Errorf("%w: fact outside column domain", ErrSieve)
		}
	}
	return nil
}

func (r *SieveRelation) projectFact(fact Fact) (Fact, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if err := r.validateFact(fact); err != nil {
		return nil, err
	}
	inner := make(Fact, len(r.innerToSig))
	for i, column := range r.innerToSig {
		inner[i] = fact[column]
	}
	return inner, nil
}

func (r *SieveRelation) AddFact(fact Fact) (bool, error) {
	inner, err := r.projectFact(fact)
	if err != nil {
		return false, err
	}
	return r.inner.Add(inner)
}

func (r *SieveRelation) ContainsFact(fact Fact) (bool, error) {
	inner, err := r.projectFact(fact)
	if err != nil {
		return false, err
	}
	return r.inner.Contains(inner)
}

func (r *SieveRelation) Empty() bool {
	if r.open() != nil {
		return true
	}
	length, err := r.inner.Len()
	return err != nil || length == 0
}

func (r *SieveRelation) NumInnerRows() int {
	if r.open() != nil {
		return 0
	}
	length, _ := r.inner.Len()
	return length
}

func (r *SieveRelation) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	return r.inner.Reset()
}

func (r *SieveRelation) Clone() (*SieveRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	inner, err := r.inner.Clone()
	if err != nil {
		return nil, err
	}
	return AdoptSieveRelation(r.signature, r.innerCols, inner)
}

// Complement complements only the retained inner coordinates. Ignored
// coordinates remain full, exactly as documented by the source operation.
func (r *SieveRelation) Complement() (*SieveRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	innerSignature, err := r.inner.Signature()
	if err != nil {
		return nil, err
	}
	// The retained table_base complement contract is compiled down to at most
	// one ordinary column; wider complements are decomposed by the caller.
	if innerSignature.Len() > 1 {
		return nil, fmt.Errorf("%w: inner complement requires at most one retained column", ErrSieve)
	}
	resultInner := NewHashTable(innerSignature)
	fact := make(Fact, innerSignature.Len())
	var enumerate func(int) error
	enumerate = func(column int) error {
		if column == len(fact) {
			found, containsErr := r.inner.Contains(fact)
			if containsErr != nil {
				return containsErr
			}
			if !found {
				_, containsErr = resultInner.Add(fact)
			}
			return containsErr
		}
		for value := uint64(0); value < innerSignature.domains[column]; value++ {
			fact[column] = Element(value)
			if err := enumerate(column + 1); err != nil {
				return err
			}
		}
		return nil
	}
	if err = enumerate(0); err != nil {
		return nil, err
	}
	return AdoptSieveRelation(r.signature, r.innerCols, resultInner)
}

func (r *SieveRelation) String() (string, error) {
	if err := r.open(); err != nil {
		return "", err
	}
	rows, err := r.inner.Facts()
	if err != nil {
		return "", err
	}
	var output strings.Builder
	output.WriteString("Sieve relation [")
	for i, selected := range r.innerCols {
		if i != 0 {
			output.WriteByte(' ')
		}
		if selected {
			output.WriteByte('1')
		} else {
			output.WriteByte('0')
		}
	}
	output.WriteString("]\n")
	for _, fact := range rows {
		fmt.Fprintln(&output, fact)
	}
	return output.String(), nil
}
