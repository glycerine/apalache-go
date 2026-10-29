package rel

// Source: src/muz/rel/dl_relation_manager.cpp finite relation/table signature
// and fact conversion paths.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// RelationManager borrows an AST manager, owns stored finite predicate tables,
// and converts between finite-domain relation values and table elements.
type RelationManager struct {
	manager   *ast.ASTManager
	relations map[ast.ASTHandle]*HashTable
	saturated map[ast.ASTHandle]bool
	closed    bool
}

func NewRelationManager(manager *ast.ASTManager) (*RelationManager, error) {
	if manager == nil {
		return nil, fmt.Errorf("%w: nil AST manager", ErrTable)
	}
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	return &RelationManager{manager: manager, relations: make(map[ast.ASTHandle]*HashTable), saturated: make(map[ast.ASTHandle]bool)}, nil
}

func (m *RelationManager) open() error {
	if m == nil || m.closed || m.manager == nil || m.relations == nil || m.saturated == nil {
		return ErrTable
	}
	return nil
}

func (m *RelationManager) MarkSaturated(predicate ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	if _, _, err := m.PredicateSignature(predicate); err != nil {
		return err
	}
	m.saturated[predicate] = true
	return nil
}

func (m *RelationManager) IsSaturated(predicate ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.saturated[predicate], nil
}

// TryGetRelation returns the borrowed table stored for predicate.
func (m *RelationManager) TryGetRelation(predicate ast.ASTHandle) (*HashTable, bool, error) {
	if err := m.open(); err != nil {
		return nil, false, err
	}
	table, found := m.relations[predicate]
	return table, found, nil
}

// GetRelation returns the borrowed table stored for predicate, creating and
// storing an empty finite table on first use.
func (m *RelationManager) GetRelation(predicate ast.ASTHandle) (*HashTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if table := m.relations[predicate]; table != nil {
		return table, nil
	}
	_, signature, err := m.PredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	table := NewHashTable(signature)
	if err = m.StoreRelation(predicate, table); err != nil {
		return nil, err
	}
	return table, nil
}

// StoreRelation transfers ownership of table to the manager. The predicate is
// retained on first insertion and the previous table is replaced.
func (m *RelationManager) StoreRelation(predicate ast.ASTHandle, table *HashTable) error {
	if err := m.open(); err != nil {
		return err
	}
	if err := table.open(); err != nil {
		return err
	}
	_, expected, err := m.PredicateSignature(predicate)
	if err != nil {
		return err
	}
	actual, err := table.Signature()
	if err != nil {
		return err
	}
	if !expected.equal(actual) {
		return fmt.Errorf("%w: stored relation signature differs from predicate", ErrTable)
	}
	if _, found := m.relations[predicate]; !found {
		if err = m.manager.IncRef(predicate); err != nil {
			return err
		}
	}
	m.relations[predicate] = table
	return nil
}

// ResetRelations releases every retained predicate and drops its owned table.
func (m *RelationManager) ResetRelations() (result error) {
	if err := m.open(); err != nil {
		return err
	}
	for predicate := range m.relations {
		consumed, err := m.manager.DecRefConsumed(predicate)
		result = errors.Join(result, err)
		if consumed {
			delete(m.relations, predicate)
			delete(m.saturated, predicate)
		}
	}
	return result
}

func (m *RelationManager) Close() (result error) {
	if m == nil {
		return ErrTable
	}
	if m.closed {
		return nil
	}
	result = m.ResetRelations()
	if len(m.relations) != 0 {
		return errors.Join(result, ErrTable)
	}
	m.closed = true
	m.manager = nil
	m.relations = nil
	m.saturated = nil
	return result
}

func (m *RelationManager) sortDomain(sort ast.ASTHandle) (uint64, error) {
	size, err := m.manager.GetSortSize(sort)
	if err != nil {
		return 0, err
	}
	if !size.IsFinite() {
		return 0, fmt.Errorf("%w: relation sort is not a bounded finite domain", ErrTable)
	}
	domain, err := size.Size()
	if err != nil {
		return 0, err
	}
	if domain == 0 {
		return 0, fmt.Errorf("%w: relation sort has an empty domain", ErrTable)
	}
	return domain, nil
}

// PredicateSignature returns borrowed domain sorts plus their concrete table
// signature. The returned sort slice itself is caller-owned.
func (m *RelationManager) PredicateSignature(declaration ast.ASTHandle) ([]ast.ASTHandle, Signature, error) {
	if err := m.open(); err != nil {
		return nil, Signature{}, err
	}
	kind, err := m.manager.GetKind(declaration)
	if err != nil {
		return nil, Signature{}, err
	}
	if kind != ast.ASTFuncDecl {
		return nil, Signature{}, fmt.Errorf("%w: predicate is not a function declaration", ErrTable)
	}
	arity, err := m.manager.GetArity(declaration)
	if err != nil {
		return nil, Signature{}, err
	}
	sorts := make([]ast.ASTHandle, arity)
	domains := make([]uint64, arity)
	for i := uint32(0); i < arity; i++ {
		sorts[i], err = m.manager.GetDomain(declaration, i)
		if err != nil {
			return nil, Signature{}, err
		}
		domains[i], err = m.sortDomain(sorts[i])
		if err != nil {
			return nil, Signature{}, err
		}
	}
	signature, err := NewSignature(domains)
	return sorts, signature, err
}

func (m *RelationManager) bitVectorElement(value ast.ASTHandle) (number uint64, matched bool, resultErr error) {
	service := m.manager.NumeralService()
	numeral, err := service.NewValue()
	if err != nil {
		return 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&numeral)) }()
	var width uint32
	matched, err = m.manager.BitVectorNumeralValue(value, &numeral, &width)
	if err != nil || !matched {
		return 0, matched, err
	}
	valid, err := service.IsUint64(&numeral)
	if err != nil {
		return 0, false, err
	}
	if !valid {
		return 0, false, fmt.Errorf("%w: bit-vector value is not a table element", ErrTable)
	}
	number, err = service.Uint64(&numeral)
	return number, true, err
}

// EncodeValue converts a concrete finite-domain value to its table element.
func (m *RelationManager) EncodeValue(sort, value ast.ASTHandle) (Element, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	domain, err := m.sortDomain(sort)
	if err != nil {
		return 0, err
	}
	actual, err := m.manager.GetSort(value)
	if err != nil {
		return 0, err
	}
	if actual != sort {
		return 0, fmt.Errorf("%w: relation value has the wrong sort", ErrTable)
	}
	number, matched, err := m.manager.DatalogNumeralValue(value)
	if err != nil {
		return 0, err
	}
	if !matched {
		number, matched, err = m.bitVectorElement(value)
		if err != nil {
			return 0, err
		}
		if !matched && m.manager.IsTrue(value) {
			number, matched = 1, true
		} else if !matched && m.manager.IsFalse(value) {
			number, matched = 0, true
		}
	}
	if !matched || number >= domain {
		return 0, fmt.Errorf("%w: expression is not a finite relation value", ErrTable)
	}
	return Element(number), nil
}

func (m *RelationManager) EncodeFact(sorts, values []ast.ASTHandle) (Fact, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if len(sorts) != len(values) {
		return nil, fmt.Errorf("%w: relation fact arity mismatch", ErrTable)
	}
	fact := make(Fact, len(values))
	for i := range values {
		value, err := m.EncodeValue(sorts[i], values[i])
		if err != nil {
			return nil, err
		}
		fact[i] = value
	}
	return fact, nil
}

// DecodeValue returns one owning AST reference for the table element.
func (m *RelationManager) DecodeValue(sort ast.ASTHandle, value Element) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	domain, err := m.sortDomain(sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if uint64(value) >= domain {
		return ast.ASTRef{}, fmt.Errorf("%w: table element outside relation sort", ErrTable)
	}
	expression, err := m.manager.MkDatalogNumeral(uint64(value), sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.manager.NewRef(expression)
}

// DecodeFact returns one owning reference per element. On failure it releases
// every reference already acquired.
func (m *RelationManager) DecodeFact(sorts []ast.ASTHandle, fact Fact) (_ []ast.ASTRef, resultErr error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if len(sorts) != len(fact) {
		return nil, fmt.Errorf("%w: table fact arity mismatch", ErrTable)
	}
	result := make([]ast.ASTRef, 0, len(fact))
	defer func() {
		if resultErr != nil {
			for i := len(result) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, m.manager.DelRef(&result[i]))
			}
		}
	}()
	for i, value := range fact {
		reference, err := m.DecodeValue(sorts[i], value)
		if err != nil {
			return nil, err
		}
		result = append(result, reference)
	}
	return result, nil
}
