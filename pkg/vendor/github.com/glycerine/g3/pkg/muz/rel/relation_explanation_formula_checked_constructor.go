package rel

// Source: src/muz/rel/rel_context.cpp magic-set fresh predicates inherit the
// relation plugin selected for their source predicate. For relation-level
// explanations, only declarations with the trailing rule-proof column use the
// product; proof-free magic predicates remain ordinary finite relations.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// NewFormulaCheckedRelationExplanationForPredicate constructs an empty outer
// formula checker around the manager-selected finite backing. eligible is
// false for proof-free auxiliary predicates.
func NewFormulaCheckedRelationExplanationForPredicate(manager *RelationManager, predicate ast.ASTHandle, config smtcore.SMTSolverConfig) (_ *RelationExplanationProduct, eligible bool, resultErr error) {
	return NewFormulaCheckedRelationExplanationForPredicateWithBacking(manager, predicate, config, "")
}

// NewFormulaCheckedRelationExplanationForPredicateWithBacking constructs an
// empty product using the selected check_relation base before installing its
// formula checker.
func NewFormulaCheckedRelationExplanationForPredicateWithBacking(manager *RelationManager, predicate ast.ASTHandle, config smtcore.SMTSolverConfig, backing string) (_ *RelationExplanationProduct, eligible bool, resultErr error) {
	if manager == nil || manager.open() != nil {
		return nil, false, ErrTableRelation
	}
	arity, err := manager.manager.GetArity(predicate)
	if err != nil {
		return nil, false, err
	}
	hasProof := false
	for column := uint32(0); column < arity; column++ {
		sort, domainErr := manager.manager.GetDomain(predicate, column)
		if domainErr != nil {
			return nil, false, domainErr
		}
		rule, ruleErr := manager.manager.IsDatalogRuleSort(sort)
		if ruleErr != nil {
			return nil, false, ruleErr
		}
		hasProof = hasProof || rule
	}
	if !hasProof {
		return nil, false, nil
	}
	sorts, explanations, err := manager.explanationPredicateSignature(predicate)
	if err != nil {
		return nil, false, fmt.Errorf("fresh explanation signature: %w", err)
	}
	if explanations != 1 {
		return nil, false, nil
	}
	if len(sorts) == 0 && (backing == "sparse" || backing == "lazy_sparse") {
		// The selected sparse plugin declines a nullary finite sieve. Relation
		// manager selection falls through to Hash without check_relation.
		return nil, false, nil
	}
	if backing == "bound_relation" || backing == "interval_relation" || backing == "karr_relation" {
		accepted, err := relationExplanationNumericCanHandle(manager, backing, sorts)
		if err != nil || !accepted {
			return nil, false, err
		}
	}
	product, err := newRelationExplanationProductWithSelectedBacking(manager, sorts, backing)
	if err != nil {
		return nil, false, fmt.Errorf("fresh explanation product: %w", err)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, product.Close())
		}
	}()
	if product.numeric != nil {
		if product.numeric.checker == nil {
			return nil, false, fmt.Errorf("%w: fresh numeric explanation product has no checker", ErrRelationVerification)
		}
		return product, true, nil
	}
	if pair, nested := product.table.(*FiniteCheckedPair); nested {
		checked, err := NewFormulaCheckedFinitePair(manager, sorts, pair, config)
		if err != nil {
			return nil, false, fmt.Errorf("fresh nested checker: %w", err)
		}
		product.table = checked
		return product, true, nil
	}
	if backing == "sparse" || backing == "lazy_sparse" {
		signature, err := product.table.Signature()
		if err != nil {
			return nil, false, err
		}
		factory, err := NewFiniteCheckedFactoryWithLazy(backing, "hashtable", manager.lazyFinite)
		if err != nil {
			return nil, false, err
		}
		pair, err := factory.New(signature)
		if err != nil {
			return nil, false, err
		}
		if err = product.table.Close(); err != nil {
			return nil, false, errors.Join(err, pair.Close())
		}
		product.table = pair
		checked, err := NewFormulaCheckedFinitePair(manager, sorts, pair, config)
		if err != nil {
			return nil, false, err
		}
		product.table = checked
		return product, true, nil
	}
	table, err := NewTableRelation(manager, sorts)
	if err != nil {
		return nil, false, err
	}
	checked, err := NewFormulaCheckedTable(table, config)
	if err != nil {
		return nil, false, errors.Join(err, table.Close())
	}
	if err = product.table.Close(); err != nil {
		return nil, false, errors.Join(err, checked.Close())
	}
	product.table = checked
	return product, true, nil
}
