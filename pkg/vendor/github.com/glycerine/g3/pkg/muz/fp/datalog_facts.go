package fp

// Source: dl_context.cpp add_fact/add_table_fact and rel_context.cpp add_fact.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *PropositionalEngine) selectFactEngine() error {
	if e.engineSelectionSet {
		return nil
	}
	if e.params == nil || e.globals == nil {
		return e.SelectEngine("auto-config")
	}
	return e.SelectConfiguredEngine()
}

func (e *PropositionalEngine) addDatalogFact(state *predicateState, fact rel.Fact) error {
	if state.table == nil {
		return ErrPropositionalHorn
	}
	if err := e.relations.ResetSaturatedMarks(); err != nil {
		return err
	}
	if _, err := state.table.Add(fact); err != nil {
		return err
	}
	e.invalidateQueryForMutation()
	e.advanceRuleRevision()
	e.compiled = false
	return nil
}

// Expression queries use private rule sets in Go. Publish only original
// relation tables back to the persistent Datalog store after successful search.
func (e *PropositionalEngine) publishDatalogQueryRelations(clone *PropositionalEngine) error {
	if clone.udocDatalog != nil {
		return nil
	}
	if clone.sparseIntervalDatalog != nil {
		return e.publishSparseIntervalFiniteRelations(clone)
	}
	for _, d := range e.predicateOrder {
		if e.auxiliary[d] {
			continue
		}
		state := clone.predicates[d]
		if state != nil && state.external != nil {
			if e.external == nil {
				return ErrPropositionalHorn
			}
			prior, existed, priorErr := e.external.TryGetPredicateRelation(d)
			if priorErr != nil {
				return priorErr
			}
			replaceErr := e.external.ReplacePredicateFrom(d, state.external)
			// Retirement can refuse after replacement publication. Refresh
			// the borrowed pointer even then, without creating a relation
			// when construction failed before publication.
			relation, found, lookupErr := e.external.TryGetPredicateRelation(d)
			if lookupErr == nil && found && (replaceErr == nil || !existed || relation != prior || e.predicates[d].external != nil) {
				e.predicates[d].external = relation
				e.predicates[d].table = nil
			}
			if err := errors.Join(replaceErr, lookupErr); err != nil {
				return err
			}
			if !found {
				return ErrPropositionalHorn
			}
			continue
		}
		if state == nil || state.table == nil {
			continue
		}
		table, err := e.cloneFiniteForContext(state.table)
		if err != nil {
			return err
		}
		if err = e.installDatalogTable(d, table); err != nil {
			return err
		}
	}
	return nil
}

// installDatalogTable consumes the incoming clone on every exit. The prior
// borrowed state table is retired only after relation-manager publication.
func (e *PropositionalEngine) installDatalogTable(d ast.ASTHandle, table rel.FiniteTable) error {
	state := e.predicates[d]
	if state == nil {
		return errors.Join(ErrPropositionalEngine, table.Close())
	}
	if err := e.relations.StoreFiniteRelation(d, table); err != nil {
		return errors.Join(err, table.Close())
	}
	state.table = table
	return nil
}
