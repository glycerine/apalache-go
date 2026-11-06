package rel

// Source: src/muz/rel/dl_mk_explanations.cpp, transform_facts.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
)

// TransferFiniteFacts adds the source fact explanation to each existing
// finite tuple and inherits predicates with facts or rules into target. The
// relation manager owns the published products under explained declarations.
// The transformer exposes borrowed views until it closes.
func (t *ExplanationTransformer) TransferFiniteFacts(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle) error {
	return t.transferFacts(source, target, relations, predicates, false, nil, smtcore.SMTSolverConfig{}, "", nil)
}

// TransferRelationFacts attaches the source fact explanation to the whole
// finite relation rather than to individual tuples.
func (t *ExplanationTransformer) TransferRelationFacts(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle) error {
	return t.transferFacts(source, target, relations, predicates, true, nil, smtcore.SMTSolverConfig{}, "", nil)
}

// TransferRelationFactsWithUDocSeeds transfers selected UDoc seed owners into
// the relation-level explanation products using typed relation union.
func (t *ExplanationTransformer) TransferRelationFactsWithUDocSeeds(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle, seeds map[ast.ASTHandle]*TypedUDocRelation) error {
	return t.transferFacts(source, target, relations, predicates, true, nil, smtcore.SMTSolverConfig{}, "", seeds)
}

// TransferRelationFactsFormulaChecked gives eligible whole-relation products a
// formula-checked finite sieve. Callers omit predicates with an explicit
// representation request; their concrete source representation is therefore
// preserved by TransferRelationFacts instead.
func (t *ExplanationTransformer) TransferRelationFactsFormulaChecked(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle, eligible map[ast.ASTHandle]bool, config smtcore.SMTSolverConfig) error {
	return t.TransferRelationFactsFormulaCheckedWithBacking(source, target, relations, predicates, eligible, config, "")
}

// TransferRelationFactsFormulaCheckedWithBacking preserves the selected
// check_relation base plugin for eligible finite explanation sieves.
func (t *ExplanationTransformer) TransferRelationFactsFormulaCheckedWithBacking(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle, eligible map[ast.ASTHandle]bool, config smtcore.SMTSolverConfig, backing string) error {
	if len(eligible) == 0 {
		return t.TransferRelationFacts(source, target, relations, predicates)
	}
	if config.Params == nil {
		return ErrTableRelation
	}
	return t.transferFacts(source, target, relations, predicates, true, eligible, config, backing, nil)
}

// TransferRelationFactsFormulaCheckedWithUDocSeeds selects the formula-checked
// DOC backing and transfers seed relations without tuple materialization.
func (t *ExplanationTransformer) TransferRelationFactsFormulaCheckedWithUDocSeeds(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle, eligible map[ast.ASTHandle]bool, config smtcore.SMTSolverConfig, backing string, seeds map[ast.ASTHandle]*TypedUDocRelation) error {
	if len(eligible) == 0 {
		return t.TransferRelationFactsWithUDocSeeds(source, target, relations, predicates, seeds)
	}
	if config.Params == nil {
		return ErrTableRelation
	}
	return t.transferFacts(source, target, relations, predicates, true, eligible, config, backing, seeds)
}

func (t *ExplanationTransformer) transferFacts(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle, relationLevel bool, formulaEligible map[ast.ASTHandle]bool, formulaConfig smtcore.SMTSolverConfig, formulaBacking string, udocSeeds map[ast.ASTHandle]*TypedUDocRelation) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || target == nil || relations == nil || relations.open() != nil || relations.manager != t.manager {
		return ast.ErrASTHandle
	}
	if t.factManager != nil && t.factManager != relations {
		return ast.ErrASTDomain
	}
	for _, set := range []*base.RuleSet{source, target} {
		manager, err := set.RuleManager()
		if err != nil {
			return err
		}
		if manager != t.rules {
			return ast.ErrASTHandle
		}
	}
	symbol, err := t.manager.Symbols.Intern("fact")
	if err != nil {
		return err
	}
	fact, err := t.manager.MkDatalogRule(symbol, nil)
	if err != nil {
		return err
	}
	factRef, err := t.manager.NewRef(fact)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, t.manager.DelRef(&factRef)) }()
	var singleton *ExplanationRelation
	if !relationLevel {
		sort, err := t.manager.MkDatalogRuleSort()
		if err != nil {
			return err
		}
		singleton, err = NewExplanationRelation(t.manager, []ast.ASTHandle{sort})
		if singleton != nil {
			defer func() { resultErr = errors.Join(resultErr, singleton.Close()) }()
		}
		if err != nil {
			return err
		}
		if err = singleton.AddFact([]ast.ASTHandle{fact}); err != nil {
			return err
		}
	}
	for _, original := range predicates {
		explained, err := t.ExplanationDeclaration(original)
		if err != nil {
			return err
		}
		var finite FiniteTable
		var exists bool
		finite, exists, err = relations.TryGetFiniteRelation(original)
		if err != nil {
			return err
		}
		udocSeed := udocSeeds[original]
		group, err := source.PredicateRules(original)
		if err != nil {
			return err
		}
		hasRules := len(group) != 0
		for i := len(group) - 1; i >= 0; i-- {
			err = errors.Join(err, group[i].Close())
		}
		if err != nil {
			return err
		}
		if !exists && udocSeed == nil && !hasRules {
			continue
		}
		if _, exists := t.factProducts[original]; exists {
			return ast.ErrASTDomain
		}
		if _, exists := t.relationProducts[original]; exists {
			return ast.ErrASTDomain
		}
		derived, err := target.DerivedPredicate(original)
		if err != nil {
			return err
		}
		if derived != explained {
			if err = target.InheritPredicate(source, original, explained); err != nil {
				return err
			}
		}
		var sorts []ast.ASTHandle
		if relationLevel {
			arity, cause := t.manager.GetArity(original)
			if cause != nil {
				return cause
			}
			sorts = make([]ast.ASTHandle, arity)
			for column := uint32(0); column < arity; column++ {
				sorts[column], cause = t.manager.GetDomain(original, column)
				if cause != nil {
					return cause
				}
			}
		} else {
			sorts, _, err = relations.PredicateSignature(original)
			if err != nil {
				return err
			}
		}
		if relationLevel {
			product, err := newRelationExplanationProductFromFiniteCheckedWithBacking(relations, sorts, finite, fact, formulaEligible[original], formulaConfig, formulaBacking)
			if err != nil {
				return err
			}
			if udocSeed != nil {
				if err = product.UnionUDocSeed(udocSeed); err != nil {
					return errors.Join(err, product.Close())
				}
			}
			if err = relations.StoreRelationExplanation(explained, product); err != nil {
				return errors.Join(err, product.Close())
			}
			t.factManager = relations
			t.relationProducts[original] = product
			continue
		}
		var product *FiniteExplanationProduct
		if exists {
			// transform_facts uses the finite-product converting join of the
			// original table with the singleton fact explanation.
			converted, err := finiteExplanationProductFromFiniteTable(relations, sorts, finite)
			if err != nil {
				return err
			}
			product, err = converted.JoinExplanationRelation(singleton)
			err = errors.Join(err, converted.Close())
			if err != nil {
				if product != nil {
					err = errors.Join(err, product.Close())
				}
				return err
			}
		} else {
			product, err = NewFiniteExplanationProduct(relations, sorts)
			if err != nil {
				return err
			}
		}
		if err = relations.StoreExplanationRelation(explained, product); err != nil {
			return errors.Join(err, product.Close())
		}
		t.factManager = relations
		t.factProducts[original] = product
	}
	return nil
}

func newRelationExplanationProductFromFinite(manager *RelationManager, sorts []ast.ASTHandle, source FiniteTable, fact ast.ASTHandle) (_ *RelationExplanationProduct, resultErr error) {
	return newRelationExplanationProductFromFiniteChecked(manager, sorts, source, fact, false, smtcore.SMTSolverConfig{})
}

func newRelationExplanationProductFromFiniteChecked(manager *RelationManager, sorts []ast.ASTHandle, source FiniteTable, fact ast.ASTHandle, formulaChecked bool, config smtcore.SMTSolverConfig) (_ *RelationExplanationProduct, resultErr error) {
	return newRelationExplanationProductFromFiniteCheckedWithBacking(manager, sorts, source, fact, formulaChecked, config, "")
}

func newRelationExplanationProductFromFiniteCheckedWithBacking(manager *RelationManager, sorts []ast.ASTHandle, source FiniteTable, fact ast.ASTHandle, formulaChecked bool, config smtcore.SMTSolverConfig, backingKind string) (_ *RelationExplanationProduct, resultErr error) {
	var product *RelationExplanationProduct
	var err error
	if formulaChecked && backingKind != "" {
		product, err = newRelationExplanationProductWithSelectedBacking(manager, sorts, backingKind)
	} else {
		product, err = newRelationExplanationProductWithBacking(manager, sorts, source)
	}
	if err != nil {
		return nil, fmt.Errorf("relation explanation finite backing: %w", err)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, product.Close())
		}
	}()
	if formulaChecked {
		if product.doc != nil {
			if !product.doc.Doc().Inner().FormulaChecked() {
				return nil, fmt.Errorf("%w: selected doc relation is not formula checked", ErrRelationVerification)
			}
		} else if product.numeric != nil {
			if product.numeric.checker == nil {
				return nil, fmt.Errorf("%w: selected numeric relation is not formula checked", ErrRelationVerification)
			}
		} else if pair, nested := product.table.(*FiniteCheckedPair); nested {
			checked, err := NewFormulaCheckedFinitePair(manager, sorts, pair, config)
			if err != nil {
				return nil, err
			}
			product.table = checked
		} else if backingKind == "sparse" || backingKind == "lazy_sparse" {
			signature, err := product.table.Signature()
			if err != nil {
				return nil, err
			}
			factory, err := NewFiniteCheckedFactoryWithLazy(backingKind, "hashtable", manager.lazyFinite)
			if err != nil {
				return nil, err
			}
			pair, err := factory.New(signature)
			if err != nil {
				return nil, err
			}
			if err = product.table.Close(); err != nil {
				return nil, errors.Join(err, pair.Close())
			}
			product.table = pair
			checked, err := NewFormulaCheckedFinitePair(manager, sorts, pair, config)
			if err != nil {
				return nil, err
			}
			product.table = checked
		} else {
			table, err := NewTableRelation(manager, sorts)
			if err != nil {
				return nil, err
			}
			checked, err := NewFormulaCheckedTable(table, config)
			if err != nil {
				return nil, errors.Join(err, table.Close())
			}
			if err = product.table.Close(); err != nil {
				return nil, errors.Join(err, checked.Close())
			}
			product.table = checked
		}
	}
	if source != nil {
		if product.doc != nil {
			if err = importFiniteFactsToExplanationUDoc(manager, sorts, source, product.doc); err != nil {
				return nil, fmt.Errorf("relation explanation doc snapshot: %w", err)
			}
			if err = product.explanation.AddFact([]ast.ASTHandle{fact}); err != nil {
				return nil, err
			}
			return product, nil
		}
		if product.numeric != nil {
			count, countErr := source.Len()
			if countErr != nil {
				return nil, countErr
			}
			if count != 0 {
				if len(sorts) != 0 {
					return nil, ErrTableRelation
				}
				seed := RelationExplanationNumericSeed{Kind: product.numeric.kind}
				switch seed.Kind {
				case "bound_relation":
					seed.Bound = NewBoundRelation(0, false)
				case "interval_relation":
					seed.Interval = NewIntervalFull(0)
				case "karr_relation":
					seed.Karr = NewKarrFull(0, nil, product.numeric.karr.formulaChecker)
				}
				if err = product.UnionNumericSeed(seed); err != nil {
					return nil, err
				}
			}
			if err = product.explanation.AddFact([]ast.ASTHandle{fact}); err != nil {
				return nil, err
			}
			return product, nil
		}
		expected, err := product.table.Signature()
		if err != nil {
			return nil, err
		}
		actual, err := source.Signature()
		if err != nil || !expected.equal(actual) {
			return nil, errors.Join(err, ErrTableRelation)
		}
		rows, err := source.Facts()
		if err != nil {
			return nil, fmt.Errorf("relation explanation source facts: %w", err)
		}
		for _, row := range rows {
			if _, err = product.table.Add(row); err != nil {
				return nil, fmt.Errorf("relation explanation snapshot add %v: %w", row, err)
			}
		}
	}
	if err = product.explanation.AddFact([]ast.ASTHandle{fact}); err != nil {
		return nil, err
	}
	return product, nil
}

// RelationProduct returns the manager-owned borrowed whole-relation
// explanation product created by TransferRelationFacts.
func (t *ExplanationTransformer) RelationProduct(original ast.ASTHandle) (*RelationExplanationProduct, bool, error) {
	if err := t.open(); err != nil {
		return nil, false, err
	}
	if _, found := t.relationProducts[original]; !found {
		return nil, false, nil
	}
	explained, err := t.ExplanationDeclaration(original)
	if err != nil {
		return nil, false, err
	}
	return t.factManager.TryGetRelationExplanation(explained)
}

// FactProduct returns a manager-owned, borrowed finite/explanation product for
// an original predicate, if TransferFiniteFacts published one.
func (t *ExplanationTransformer) FactProduct(original ast.ASTHandle) (*FiniteExplanationProduct, bool, error) {
	if err := t.open(); err != nil {
		return nil, false, err
	}
	if _, found := t.factProducts[original]; !found {
		return nil, false, nil
	}
	explained, err := t.ExplanationDeclaration(original)
	if err != nil {
		return nil, false, err
	}
	return t.factManager.TryGetExplanationRelation(explained)
}
