package fp

// Source: dl_relation_manager.cpp register_relation_plugin_impl chooses the
// favourite during engine construction; interval_relation is not the default
// arithmetic fallback (bound_relation precedes it in registration order).

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (e *PropositionalEngine) cacheDatalogRelationFavourite() error {
	if e.datalogRelationFavourite != "" {
		return nil
	}
	name := "pentagon"
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return err
		}
		selected, err := view.Symbol("datalog.default_relation", symbol.Symbol{})
		if err == nil && selected.Value != 0 {
			name, _, err = e.manager.Symbols.BareString(selected)
		}
		if err = errors.Join(err, view.Close()); err != nil {
			return err
		}
	}
	e.datalogRelationFavourite = name
	return nil
}

func (e *PropositionalEngine) ensureIntervalDatalog() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	checkName, checkEnabled, err := e.datalogCheckRelationName()
	if err != nil {
		return false, err
	}
	checkerRequests := checkEnabled && checkName == "interval_relation"
	representationRequested := false
	for _, state := range e.predicates {
		representationRequested = representationRequested || len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "interval_relation"
	}
	if (!checkerRequests && !representationRequested && e.datalogRelationFavourite != "interval_relation") || len(e.predicateOrder) == 0 {
		return false, nil
	}
	// Without an interval favourite, every original must request this kind;
	// generated query heads inherit the selected relation.
	if representationRequested && e.datalogRelationFavourite != "interval_relation" {
		for d, state := range e.predicates {
			if !e.auxiliary[d] && (len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != "interval_relation") {
				return true, ErrIntervalDatalogFragment
			}
		}
	}
	for _, state := range e.predicates {
		if state.explicitFiniteRepresentation || state.external != nil {
			return true, ErrIntervalDatalogFragment
		}
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return true, errors.Join(err, ErrIntervalDatalogFragment)
	}
	if checkEnabled && checkName != "interval_relation" {
		return true, errors.Join(err, ErrIntervalDatalogFragment)
	}
	var formulaCheck *smtcore.SMTSolverConfig
	if checkEnabled {
		formulaCheck = &smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrIntervalDatalogFragment)
	}
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		magic, err := view.Bool("datalog.magic_sets_for_queries", false)
		err = errors.Join(err, view.Close())
		if err != nil || magic {
			return true, errors.Join(err, ErrIntervalDatalogFragment)
		}
	}
	if e.intervalDatalog != nil {
		e.intervalDatalog.formulaCheck = formulaCheck
		return true, e.intervalDatalog.syncPredicates(e.predicateOrder)
	}
	owner, selected, err := newIntervalDatalogActivation(e.manager, e.set, e.predicateOrder)
	if err != nil {
		return true, err
	}
	if !selected {
		return true, ErrIntervalDatalogFragment
	}
	owner.formulaCheck = formulaCheck
	if err = owner.verifyEmptySeeds(); err != nil {
		return true, errors.Join(err, owner.close())
	}
	e.intervalDatalog = owner
	// Any eagerly stored nullary tuples predate activation. Copy them into the
	// selected imprecise relation without changing their source fact meaning.
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if state.table != nil && len(state.sorts) == 0 {
			count, err := state.table.Len()
			if err != nil {
				return true, err
			}
			if count != 0 {
				if err = owner.addFact(d, nil); err != nil {
					return true, err
				}
			}
		}
	}
	return true, nil
}

func (e *PropositionalEngine) queryIntervalDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	widen := false
	similarity, threshold := true, uint32(11)
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return lbool.Undef, err
		}
		widen, err = view.Bool("datalog.compile_with_widening", false)
		if err == nil {
			similarity, err = view.Bool("datalog.similarity_compressor", true)
		}
		if err == nil && similarity {
			threshold, err = view.UInt("datalog.similarity_compressor_threshold", 11)
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return lbool.Undef, err
		}
	}
	e.compiled = false
	e.datalogStats.Reset()
	if similarity {
		return e.intervalDatalog.queryWithSimilarity(declarations, widen, threshold)
	}
	return e.intervalDatalog.query(declarations, widen)
}

func (e *PropositionalEngine) closeIntervalDatalog() error {
	if e.intervalDatalog == nil {
		return nil
	}
	if err := e.intervalDatalog.close(); err != nil {
		return err
	}
	e.intervalDatalog = nil
	return nil
}
