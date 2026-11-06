package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp query/query_from_lvl local
// rule-set copies and src/muz/pdr/pdr_dl_interface.cpp query local rules.

import "errors"

func (e *PropositionalEngine) newQueryClone() (*PropositionalEngine, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.backend == "datalog" && (e.externalIntervalDatalog != nil || e.hasExternalIntervalRepresentation()) {
		return nil, ErrExternalIntervalDatalogFragment
	}
	if err := e.clearQueryCleanup(); err != nil {
		return nil, err
	}
	var clone *PropositionalEngine
	var err error
	if e.params != nil && e.globals != nil {
		clone, err = NewPropositionalEngineWithServices(e.manager, e.params, e.globals)
	} else {
		clone, err = NewPropositionalEngine(e.manager)
	}
	if err != nil {
		return nil, err
	}
	// PDR queries on a temporary rule-set clone still borrow the original
	// fixedpoint context's already configured source rewriter.
	clone.pdrTheory = e.pdrTheory
	if clone.pdrTheory == nil {
		clone.pdrTheory = e.guard
	}
	clone.backend = e.backend
	clone.engineSelectionSet = e.engineSelectionSet
	if e.backend == EngineTab.String() && e.tabSession != nil {
		clone.tabSession = e.tabSession
		clone.tabSessionBorrowed = true
	}
	clone.datalogRelationFavourite = e.datalogRelationFavourite
	clone.datalogDefaultTable = e.datalogDefaultTable
	if clone.datalogDefaultTable != "" {
		if err = clone.configureDatalogFiniteDefault(clone.datalogDefaultTable); err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if e.queryParamsReady {
		if err = clone.SetQueryParameters(&e.queryParams); err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	clone.ddnf.output = e.ddnf.output
	if e.ddnf.registry != nil {
		clone.ddnf.registry = e.ddnf.registry
		clone.ddnf.registryBorrowed = true
	}
	clone.forceProofTrace = e.forceProofTrace
	if e.backend == "datalog" && e.external != nil {
		clone.external, err = e.external.CloneForManager(clone.relations)
		if err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if err = e.copyQueryPredicates(clone); err != nil {
		return nil, errors.Join(err, e.discardQueryClone(clone))
	}
	if err = e.copyQueryRules(clone); err != nil {
		return nil, errors.Join(err, e.discardQueryClone(clone))
	}
	if e.udocDatalog != nil {
		clone.udocDatalog, err = e.udocDatalog.clone()
		if err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
		if err = clone.udocDatalog.sync(clone); err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if e.karrDatalog != nil {
		clone.karrDatalog, err = e.karrDatalog.clone(clone.set)
		if err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if e.intervalDatalog != nil {
		clone.intervalDatalog, err = e.intervalDatalog.clone(clone.set)
		if err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if e.boundDatalog != nil {
		clone.boundDatalog, err = e.boundDatalog.clone(clone.set)
		if err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if e.boundIntervalDatalog != nil {
		clone.boundIntervalDatalog, err = e.boundIntervalDatalog.clone(clone.set)
		if err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if e.sparseIntervalDatalog != nil {
		clone.sparseIntervalDatalog, err = e.sparseIntervalDatalog.clone(clone.set, clone.guard)
		if err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if err = e.copyQueryBackground(clone); err != nil {
		return nil, errors.Join(err, e.discardQueryClone(clone))
	}
	return clone, nil
}
