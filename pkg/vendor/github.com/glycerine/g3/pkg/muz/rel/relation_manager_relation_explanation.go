package rel

// Source: src/muz/rel/dl_relation_manager.cpp typed relation ownership and
// src/muz/rel/dl_mk_explanations.cpp relation-level explanation products.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (m *RelationManager) relationExplanationPredicateSignature(predicate ast.ASTHandle) ([]ast.ASTHandle, ast.ASTHandle, Signature, error) {
	sorts, explanations, err := m.explanationPredicateSignature(predicate)
	if err != nil {
		return nil, ast.ASTHandle{}, Signature{}, err
	}
	if explanations != 1 {
		return nil, ast.ASTHandle{}, Signature{}, ErrTableRelation
	}
	arity, err := m.manager.GetArity(predicate)
	if err != nil {
		return nil, ast.ASTHandle{}, Signature{}, err
	}
	proofSort, err := m.manager.GetDomain(predicate, arity-1)
	if err != nil {
		return nil, ast.ASTHandle{}, Signature{}, err
	}
	if m.explanationNumericKind != "" {
		accepted, err := relationExplanationNumericCanHandle(m, m.explanationNumericKind, sorts)
		if err != nil {
			return nil, ast.ASTHandle{}, Signature{}, err
		}
		if accepted {
			return sorts, proofSort, Signature{}, nil
		}
	}
	domains := make([]uint64, len(sorts))
	for i, sort := range sorts {
		domains[i], err = m.sortDomain(sort)
		if err != nil {
			return nil, ast.ASTHandle{}, Signature{}, err
		}
	}
	signature, err := NewSignature(domains)
	return sorts, proofSort, signature, err
}

func (m *RelationManager) validateRelationExplanation(predicate ast.ASTHandle, product *RelationExplanationProduct) ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrTableRelation
	}
	if err := product.open(); err != nil {
		return nil, err
	}
	sorts, proofSort, expected, err := m.relationExplanationPredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	if product.numeric != nil {
		accepted, err := relationExplanationNumericCanHandle(m, product.numeric.kind, sorts)
		if err != nil || !accepted || m.explanationNumericKind != product.numeric.kind {
			return nil, errors.Join(err, ErrTableRelation)
		}
	} else {
		actual, err := product.FiniteSignature()
		if err != nil || !expected.equal(actual) {
			return nil, errors.Join(err, ErrTableRelation)
		}
	}
	if product.manager != m || len(sorts) != len(product.finiteSorts) {
		return nil, ErrTableRelation
	}
	for i := range sorts {
		if sorts[i] != product.finiteSorts[i] {
			return nil, ErrTableRelation
		}
	}
	proofSignature, err := product.explanation.signature()
	if err != nil || len(proofSignature) != 1 || proofSignature[0] != proofSort {
		return nil, errors.Join(err, ErrTableRelation)
	}
	return sorts, nil
}

// TryGetRelationExplanation borrows a stored whole-relation explanation
// product. Its proof is stored internally, while the storage declaration has
// the corresponding trailing rule-sort column.
func (m *RelationManager) TryGetRelationExplanation(predicate ast.ASTHandle) (*RelationExplanationProduct, bool, error) {
	if err := m.open(); err != nil {
		return nil, false, err
	}
	product, found := m.relationLevel[predicate]
	if product != nil {
		if err := product.open(); err != nil {
			return nil, false, err
		}
	}
	return product, found, nil
}

// GetRelationExplanation returns the borrowed product, constructing an empty
// product for the predicate's finite signature on first use.
func (m *RelationManager) GetRelationExplanation(predicate ast.ASTHandle) (*RelationExplanationProduct, error) {
	product, found, err := m.TryGetRelationExplanation(predicate)
	if err != nil || found {
		return product, err
	}
	sorts, _, _, err := m.relationExplanationPredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	product, err = NewRelationExplanationProduct(m, sorts)
	if err != nil {
		return nil, err
	}
	if err = m.StoreRelationExplanation(predicate, product); err != nil {
		return nil, errors.Join(err, product.Close())
	}
	return product, nil
}

// StoreRelationExplanation consumes product on success. Replacement closes
// the former owner and preserves the manager's predicate reference.
func (m *RelationManager) StoreRelationExplanation(predicate ast.ASTHandle, product *RelationExplanationProduct) error {
	if _, err := m.validateRelationExplanation(predicate, product); err != nil {
		return err
	}
	if m.relations[predicate] != nil || m.finite[predicate] != nil || m.explanations[predicate] != nil {
		return ErrTableRelation
	}
	prior, found := m.relationLevel[predicate]
	if prior == product {
		return nil
	}
	if found {
		if err := prior.Close(); err != nil {
			return err
		}
	} else if err := m.manager.IncRef(predicate); err != nil {
		return err
	}
	m.relationLevel[predicate] = product
	return nil
}

// ExchangeRelationExplanation transfers replacement to the manager and the
// existing owner to the caller without closing either or changing predicate
// reference ownership. On failure replacement remains caller-owned.
func (m *RelationManager) ExchangeRelationExplanation(predicate ast.ASTHandle, replacement *RelationExplanationProduct) (*RelationExplanationProduct, error) {
	if _, err := m.validateRelationExplanation(predicate, replacement); err != nil {
		return nil, err
	}
	prior := m.relationLevel[predicate]
	if prior == nil || prior == replacement {
		return nil, ErrTableRelation
	}
	if err := prior.open(); err != nil {
		return nil, err
	}
	m.relationLevel[predicate] = replacement
	return prior, nil
}
