package fp

// Source: rel_context::updt_params selects check_relation(doc) as the
// favourite. Execution stays in the concrete UDoc compiler and storage.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
)

func (e *PropositionalEngine) newUDocDatalogOwner(checked bool) (*datalogUDocContext, error) {
	if !checked {
		return newDatalogUDocContext(e.manager)
	}
	factory, err := rel.NewFormulaCheckedUDocFactory(e.manager, arena.VectorConfig{}, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return nil, err
	}
	return &datalogUDocContext{manager: e.manager, factory: factory, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation)}, nil
}

func (c *datalogUDocContext) importFiniteFact(e *PropositionalEngine, relation *rel.TypedUDocRelation, sorts []ast.ASTHandle, fact rel.Fact) (resultErr error) {
	if !relation.FormulaChecked() {
		values := make([]uint64, len(fact))
		for i, value := range fact {
			values[i] = uint64(value)
		}
		_, err := relation.Core().AddFact(values)
		return err
	}
	attempt := &storedFactFormulaAttempt{manager: c.manager}
	defer func() { resultErr = attempt.fail(resultErr) }()
	values, err := e.relations.DecodeFact(sorts, fact)
	attempt.current = values
	if err != nil {
		return err
	}
	handles := make([]ast.ASTHandle, len(values))
	for i, value := range values {
		handles[i], err = c.manager.RefGet(value)
		if err != nil {
			return err
		}
	}
	_, err = relation.AddFact(handles)
	return err
}

func (e *PropositionalEngine) beginDatalogDocNullaries() (_ *datalogCheckRelationScope, resultErr error) {
	factory, err := rel.NewFormulaCheckedUDocFactory(e.manager, arena.VectorConfig{}, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return nil, err
	}
	scope := &datalogCheckRelationScope{engine: e, docFactory: factory}
	defer func() {
		if resultErr != nil {
			errors.As(resultErr, &scope.docNested)
			if cleanup := scope.dispose(); cleanup != nil {
				resultErr = &datalogDocScopeConstructionFailure{original: resultErr, cleanup: cleanup, scope: scope, reported: errors.Join(resultErr, cleanup)}
			}
		}
	}()
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil || state.table == nil || len(state.sorts) != 0 || len(state.requestedRepresentations) != 0 {
			continue
		}
		doc, err := rel.NewUDocNullaryTable(factory)
		if err != nil {
			return nil, err
		}
		scope.entries = append(scope.entries, datalogCheckedPredicate{declaration: declaration, state: state, original: state.table, doc: doc})
		facts, err := state.table.Facts()
		if err != nil {
			return nil, err
		}
		for _, fact := range facts {
			if _, err = doc.Add(fact); err != nil {
				return nil, err
			}
		}
		state.table = doc
	}
	return scope, nil
}

func (e *PropositionalEngine) legacyMixedDocChecker() (bool, error) {
	factory, err := rel.NewUDocRelationFactory(e.manager, arena.VectorConfig{})
	if err != nil {
		return false, err
	}
	defer factory.Close()
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil || len(state.sorts) == 0 {
			continue
		}
		supported, err := factory.CanHandleSignature(state.sorts)
		if err != nil {
			return false, err
		}
		if supported {
			return true, nil
		}
	}
	return false, nil
}

type datalogDocScopeConstructionFailure struct {
	original, cleanup, reported error
	scope                       *datalogCheckRelationScope
}

func (f *datalogDocScopeConstructionFailure) Error() string        { return f.reported.Error() }
func (f *datalogDocScopeConstructionFailure) Unwrap() error        { return f.reported }
func (f *datalogDocScopeConstructionFailure) OriginalCause() error { return f.original }
func (f *datalogDocScopeConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *datalogDocScopeConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.scope.engine.manager.NumeralService().RuntimeOwner()
}
func (f *datalogDocScopeConstructionFailure) Cleanup() error {
	f.cleanup = f.scope.dispose()
	return f.cleanup
}

var _ memory.ConstructionFailure = (*datalogDocScopeConstructionFailure)(nil)
