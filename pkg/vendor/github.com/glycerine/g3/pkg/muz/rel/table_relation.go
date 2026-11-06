package rel

// Source: src/muz/rel/dl_table_relation.h and the table_relation methods in
// dl_table_relation.cpp. This is the retained concrete HashTable-backed
// relation specialization; plugin selection remains in RelationManager.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrTableRelation = errors.New("muz/rel: invalid table relation operation")

// TableRelation owns its table and borrows the RelationManager and sort
// handles. The source scoped_rel ownership is represented by exclusive table
// access through this wrapper.
type TableRelation struct {
	manager *RelationManager
	sorts   []ast.ASTHandle
	table   *HashTable
	closed  bool
}

func tableRelationSignature(manager *RelationManager, sorts []ast.ASTHandle) (Signature, error) {
	if manager == nil || manager.open() != nil {
		return Signature{}, ErrTableRelation
	}
	domains := make([]uint64, len(sorts))
	for i, sort := range sorts {
		domain, err := manager.sortDomain(sort)
		if err != nil {
			return Signature{}, err
		}
		domains[i] = domain
	}
	return NewSignature(domains)
}

func NewTableRelation(manager *RelationManager, sorts []ast.ASTHandle) (*TableRelation, error) {
	signature, err := tableRelationSignature(manager, sorts)
	if err != nil {
		return nil, err
	}
	return AdoptTableRelation(manager, sorts, NewHashTable(signature))
}

// NewPredicateTableRelation constructs an empty relation from a predicate's
// domain signature.
func NewPredicateTableRelation(manager *RelationManager, predicate ast.ASTHandle) (*TableRelation, error) {
	if manager == nil {
		return nil, ErrTableRelation
	}
	sorts, _, err := manager.PredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	return NewTableRelation(manager, sorts)
}

// AdoptTableRelation transfers ownership of table to the result.
func AdoptTableRelation(manager *RelationManager, sorts []ast.ASTHandle, table *HashTable) (*TableRelation, error) {
	if table == nil || table.open() != nil {
		return nil, ErrTableRelation
	}
	expected, err := tableRelationSignature(manager, sorts)
	if err != nil {
		return nil, err
	}
	actual, err := table.Signature()
	if err != nil {
		return nil, err
	}
	if !expected.equal(actual) {
		return nil, fmt.Errorf("%w: relation and table signatures differ", ErrTableRelation)
	}
	return &TableRelation{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...), table: table}, nil
}

func (r *TableRelation) open() error {
	if r == nil || r.closed || r.manager == nil || r.manager.open() != nil || r.table == nil || r.table.open() != nil {
		return ErrTableRelation
	}
	return nil
}

func (r *TableRelation) Close() error {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true
	r.manager = nil
	r.sorts = nil
	r.table = nil
	return nil
}

func (r *TableRelation) Sorts() []ast.ASTHandle {
	if r.open() != nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), r.sorts...)
}

func (r *TableRelation) Signature() (Signature, error) {
	if err := r.open(); err != nil {
		return Signature{}, err
	}
	return r.table.Signature()
}

// Table returns the borrowed concrete table for retained low-level consumers.
func (r *TableRelation) Table() *HashTable {
	if r.open() != nil {
		return nil
	}
	return r.table
}

func (r *TableRelation) Empty() bool {
	if r.open() != nil {
		return true
	}
	length, err := r.table.Len()
	return err != nil || length == 0
}

func (r *TableRelation) NumRows() int {
	if r.open() != nil {
		return 0
	}
	length, _ := r.table.Len()
	return length
}

func (r *TableRelation) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	return r.table.Reset()
}

func (r *TableRelation) AddTableFact(fact Fact) (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return r.table.Add(fact)
}

func (r *TableRelation) AddFact(values []ast.ASTHandle) (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	fact, err := r.manager.EncodeFact(r.sorts, values)
	if err != nil {
		return false, err
	}
	return r.table.Add(fact)
}

func (r *TableRelation) ContainsFact(values []ast.ASTHandle) (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	fact, err := r.manager.EncodeFact(r.sorts, values)
	if err != nil {
		return false, err
	}
	return r.table.Contains(fact)
}

func (r *TableRelation) Clone() (*TableRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	table, err := r.table.Clone()
	if err != nil {
		return nil, err
	}
	return AdoptTableRelation(r.manager, r.sorts, table)
}

// Complement retains table_base's source contract: the compiler invokes it
// only for zero- or one-column tables and decomposes wider complements.
func (r *TableRelation) Complement() (*TableRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	result, err := r.table.Complement()
	if err != nil {
		if errors.Is(err, ErrTable) {
			return nil, fmt.Errorf("%w: %v", ErrTableRelation, err)
		}
		return nil, err
	}
	return AdoptTableRelation(r.manager, r.sorts, result)
}

func (r *TableRelation) Facts() ([]Fact, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return r.table.Facts()
}
