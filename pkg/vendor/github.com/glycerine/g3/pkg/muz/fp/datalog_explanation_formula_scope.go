package fp

// Source: rel_context::init and relation_manager::mk_empty_relation select
// check_relation as the favourite only when no explicit relation kind applies.
// Explanation products keep their proof component outside this finite sieve.
import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
)

type datalogExplanationFormulaScope struct {
	engine   *PropositionalEngine
	eligible map[ast.ASTHandle]bool
	nested   bool
	backing  string
}

func (e *PropositionalEngine) beginDatalogExplanationFormulaScope() (*datalogExplanationFormulaScope, bool, error) {
	name, enabled, err := e.datalogCheckRelationName()
	if err != nil || !enabled {
		return nil, false, err
	}
	// check_relation wraps its selected finite plugin. Preserve supported
	// concrete finite backings through the relation-level explanation sieve.
	if name != "hashtable" && name != "sparse" && name != "lazy_sparse" && name != "doc" && name != "check" && name != "bound_relation" && name != "interval_relation" && name != "karr_relation" {
		return nil, false, fmt.Errorf("%w: explanation formula checker %s", rel.ErrCompilerUnsupported, name)
	}
	backing := name
	if name == "check" {
		backing = ""
	}
	if name == "lazy_sparse" && e.lazyFinite == nil {
		e.lazyFinite = rel.NewLazySparseFiniteFactory()
		if err = e.relations.SetLazySparseFiniteFactory(e.lazyFinite); err != nil {
			return nil, false, err
		}
	}
	if name == "doc" {
		if _, found, factoryErr := e.relations.RelationExplanationUDocFactory(); factoryErr != nil {
			return nil, false, factoryErr
		} else if !found {
			var factory *rel.UDocRelationFactory
			if e.udocDatalog != nil && e.udocDatalog.factory.FormulaChecked() {
				factory = e.udocDatalog.factory
			} else {
				factory, factoryErr = rel.NewFormulaCheckedUDocFactory(e.manager, arena.VectorConfig{}, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
				if factoryErr != nil {
					return nil, false, factoryErr
				}
			}
			borrowed := factory == e.udocDatalogFactory()
			if borrowed {
				factoryErr = e.relations.BorrowRelationExplanationUDocFactory(factory)
			} else {
				factoryErr = e.relations.SetRelationExplanationUDocFactory(factory)
			}
			if factoryErr != nil {
				if !borrowed {
					factory.Close()
				}
				return nil, false, factoryErr
			}
		}
	}
	if name == "bound_relation" || name == "interval_relation" || name == "karr_relation" {
		config := smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
		if err = e.relations.SetRelationExplanationNumericBacking(name, &config); err != nil {
			return nil, false, err
		}
	}
	scope := &datalogExplanationFormulaScope{engine: e, eligible: make(map[ast.ASTHandle]bool), nested: name == "check", backing: backing}
	var udoc *rel.UDocRelationFactory
	if backing == "doc" {
		udoc, _, err = e.relations.RelationExplanationUDocFactory()
		if err != nil {
			return nil, false, err
		}
	}
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state != nil {
			eligible := len(state.requestedRepresentations) == 0
			if eligible && udoc != nil {
				// check_relation delegates capability to its selected base. When
				// DOC declines a whole signature, the relation manager selects its
				// ordinary fallback and no DOC formula check is installed.
				eligible, err = udoc.CanHandleSignature(state.sorts)
				if err != nil {
					return nil, false, err
				}
			}
			if eligible && (backing == "bound_relation" || backing == "interval_relation" || backing == "karr_relation") {
				eligible, err = rel.RelationExplanationNumericCanHandle(e.relations, backing, state.sorts)
				if err != nil {
					return nil, false, err
				}
			}
			// Sparse families reject the nullary signature. The source relation
			// manager then falls through to ordinary Hash without this wrapper.
			if (backing == "sparse" || backing == "lazy_sparse") && len(state.sorts) == 0 {
				eligible = false
			}
			scope.eligible[declaration] = eligible
		}
	}
	return scope, true, nil
}

func (s *datalogExplanationFormulaScope) transferFacts(transformer *rel.ExplanationTransformer, source, target *base.RuleSet) error {
	if s == nil || s.engine == nil {
		return rel.ErrCompilerUnsupported
	}
	e := s.engine
	if s.backing == "doc" && e.udocDatalog != nil {
		return transformer.TransferRelationFactsFormulaCheckedWithUDocSeeds(source, target, e.relations, e.predicateOrder, s.eligible, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}, s.backing, e.udocDatalog.seeds)
	}
	return transformer.TransferRelationFactsFormulaCheckedWithBacking(source, target, e.relations, e.predicateOrder, s.eligible, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}, s.backing)
}

func (e *PropositionalEngine) udocDatalogFactory() *rel.UDocRelationFactory {
	if e != nil && e.udocDatalog != nil {
		return e.udocDatalog.factory
	}
	return nil
}

func (s *datalogExplanationFormulaScope) installFreshProducts(predicates []ast.ASTHandle) error {
	if s == nil || s.engine == nil {
		return rel.ErrCompilerUnsupported
	}
	e := s.engine
	config := smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
	for _, declaration := range predicates {
		_, found, err := e.relations.TryGetRelationExplanation(declaration)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		product, eligible, err := rel.NewFormulaCheckedRelationExplanationForPredicateWithBacking(e.relations, declaration, config, s.backing)
		if err != nil {
			return fmt.Errorf("muz/fp: construct fresh explanation product %v: %w", declaration, err)
		}
		if !eligible {
			continue
		}
		if err = e.relations.StoreRelationExplanation(declaration, product); err != nil {
			return errors.Join(fmt.Errorf("muz/fp: store fresh explanation product %v: %w", declaration, err), product.Close())
		}
	}
	return nil
}

func (s *datalogExplanationFormulaScope) requireProduct(original ast.ASTHandle, product *rel.RelationExplanationProduct) error {
	if s == nil || s.engine == nil {
		return rel.ErrCompilerUnsupported
	}
	eligible, known := s.eligible[original]
	if !known {
		return fmt.Errorf("%w: predicate missing from explanation checker selection", rel.ErrRelationVerification)
	}
	if !eligible {
		return nil
	}
	return s.requireConcreteProduct(product)
}

func (s *datalogExplanationFormulaScope) requireConcreteProduct(product *rel.RelationExplanationProduct) error {
	if s == nil || s.engine == nil {
		return rel.ErrCompilerUnsupported
	}
	checked, err := product.FormulaChecked()
	if err != nil {
		return err
	}
	if !checked {
		return fmt.Errorf("%w: explanation finite sieve was not formula checked", rel.ErrRelationVerification)
	}
	return nil
}

// Close releases borrowed query-selection metadata. Products own their
// concrete checker and formula independently; no manager-global selection is
// changed by this scope.
func (s *datalogExplanationFormulaScope) Close() error {
	if s != nil {
		s.engine = nil
		s.eligible = nil
	}
	return nil
}
