package rel

import (
	"errors"
	"fmt"
)

type CheckedRelation struct {
	signature Signature
	primary   *HashTable
	checker   *SparseTable
	closed    bool
}

func NewCheckedRelation(signature Signature) (*CheckedRelation, error) {
	s, e := NewSparseTable(signature, 0)
	if e != nil {
		return nil, e
	}
	r := &CheckedRelation{signature: signature, primary: NewHashTable(signature), checker: s}
	return r, nil
}

// NewCheckedRelationFromTable copies a concrete finite relation into the
// paired hashtable/sparse checker used by datalog.check_relation=sparse.
func NewCheckedRelationFromTable(source *HashTable) (_ *CheckedRelation, resultErr error) {
	if source == nil || source.open() != nil {
		return nil, ErrTable
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	result, err := NewCheckedRelation(signature)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := source.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Primary returns the borrowed hashtable side used by the finite compiler.
func (r *CheckedRelation) Primary() (*HashTable, error) {
	if err := r.verify("primary"); err != nil {
		return nil, err
	}
	return r.primary, nil
}
func (r *CheckedRelation) open() error {
	if r == nil || r.closed || r.primary == nil || r.checker == nil {
		return ErrTable
	}
	return nil
}
func (r *CheckedRelation) Close() error {
	if r == nil {
		return ErrTable
	}
	if r.closed {
		return nil
	}
	e := errors.Join(r.primary.Close(), r.checker.Close())
	r.primary = nil
	r.checker = nil
	r.closed = true
	return e
}
func (r *CheckedRelation) fail(objective string) error {
	return fmt.Errorf("%w: check_relation %s mismatch", ErrTable, objective)
}
func (r *CheckedRelation) Len() (int, error) {
	if e := r.open(); e != nil {
		return 0, e
	}
	n, e := r.primary.Len()
	if e != nil {
		return 0, e
	}
	m, e := r.checker.Len()
	if e != nil {
		return 0, e
	}
	if n != m {
		return 0, r.fail("size")
	}
	return n, nil
}
func (r *CheckedRelation) Facts() ([]Fact, error) {
	if e := r.verify("facts"); e != nil {
		return nil, e
	}
	return r.primary.Facts()
}
