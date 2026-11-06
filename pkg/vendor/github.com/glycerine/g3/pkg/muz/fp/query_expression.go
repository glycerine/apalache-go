package fp

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_query and
// src/muz/spacer/spacer_dl_interface.cpp dl_interface::query.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// QueryExpression evaluates an arbitrary Boolean Horn query without adding
// its synthetic query predicate or rule to the caller's persistent program.
func (e *PropositionalEngine) QueryExpression(query ast.ASTHandle) (lbool.LBool, error) {
	return e.queryExpression(query, nil)
}

// QueryExpressionFromLevel runs a general Spacer expression query beginning
// at the requested proof level without changing the persistent rule set.
func (e *PropositionalEngine) QueryExpressionFromLevel(query ast.ASTHandle, level uint32) (lbool.LBool, error) {
	return e.queryExpression(query, &level)
}

func (e *PropositionalEngine) queryExpression(query ast.ASTHandle, level *uint32) (result lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	// Source context::query resolves the configured engine before choosing
	// raw-rule flushing or query_from_lvl dispatch.
	if err := e.selectFactEngine(); err != nil {
		return lbool.Undef, err
	}
	if level != nil && e.backend != "spacer" {
		return lbool.Undef, ErrPropositionalEngine
	}
	// Every failure after a query begins invalidates the previous snapshot,
	// including normalization, clone construction, and deferred cleanup.
	defer func() {
		if resultErr != nil {
			e.recordQueryStatus(result, resultErr)
		}
	}()
	// The source mixed product has no well-sorted Boolean formula answer: its
	// external component is relation-sorted while the interval component is
	// Boolean. Reject before allocating a query clone or invoking callbacks.
	if e.backend == "datalog" && e.hasExternalIntervalRepresentation() {
		return lbool.Undef, ErrExternalIntervalDatalogFragment
	}
	if err := e.clearQueryResources(); err != nil {
		return lbool.Undef, err
	}
	if e.backend == "bmc" || e.backend == "qbmc" {
		// bmc::query resets its solver before query-rule preparation. A
		// failure before clone construction must not expose the prior solve.
		e.bmcStatistics.Reset()
	}
	if e.backend == "datalog" {
		if err := e.cacheDatalogRelationFavourite(); err != nil {
			return lbool.Undef, err
		}
	}
	var originalTraceQuery ast.ASTRef
	if (e.backend == "pdr" || e.backend == "qpdr") && e.manager.BorrowSourceTraceTags().ShouldTrace("pdr") {
		ref, err := e.manager.NewRef(query)
		if err != nil {
			return lbool.Undef, err
		}
		originalTraceQuery = ref
		defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&originalTraceQuery)) }()
	}
	if e.backend == "duality" {
		e.queried = true
		result, err := e.queryDuality(query)
		e.recordQueryStatus(result, err)
		return result, err
	}
	if err := e.flushPendingRules(); err != nil {
		return lbool.Undef, err
	}
	// Source context::query calls ensure_engine before dl_interface::query
	// reaches mk_query. PDR and Spacer construct their actual contexts in the
	// interface constructor, so a later hoist/compaction failure still forbids
	// Pop. A failed constructor must not publish that phase. Existing contexts
	// are checked for rule replacement when transferred into the query clone.
	switch e.backend {
	case "pdr", "qpdr":
		if e.pdr == nil {
			if _, err := e.pdrContext(); err != nil {
				return lbool.Undef, err
			}
		}
		e.engineInstantiated = true
	case "spacer":
		if e.spacer == nil {
			if _, err := e.spacerContext(); err != nil {
				return lbool.Undef, err
			}
		}
		e.engineInstantiated = true
	}
	hoisted, err := e.hoistQueryExistentials(query)
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: hoist query existentials: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&hoisted)) }()
	query, err = e.manager.RefGet(hoisted)
	if err != nil {
		return lbool.Undef, err
	}
	compacted, err := e.compactQueryVariables(query)
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: compact query variables: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&compacted)) }()
	query, err = e.manager.RefGet(compacted)
	if err != nil {
		return lbool.Undef, err
	}
	e.queried = true
	clone, err := e.newQueryClone()
	if err != nil {
		return lbool.Undef, err
	}
	// Install or discard the expression-query clone before recording status so
	// bounded Spacer state is read from the owner that performed the search.
	defer func() { e.recordQueryStatus(result, resultErr) }()
	var prequeryPDR *pdr.Context
	var prequerySpacer *spacer.Context
	defer func() {
		if e.backend == EngineTab.String() && e.tabSession == nil && clone.tabSession != nil {
			e.tabSession = clone.tabSession
			e.tabSessionBorrowed = false
			clone.tabSession = nil
			clone.tabSessionBorrowed = false
		}
		if e.backend == "tab" {
			e.tabUnfold += clone.tabUnfold
			e.tabUnfoldFail += clone.tabUnfoldFail
			e.tabSubsumed += clone.tabSubsumed
		}
		if e.backend == "tab" && resultErr == nil {
			if err := e.clearTabAnswer(); err != nil {
				resultErr = errors.Join(resultErr, err)
			} else {
				e.tabAnswer = clone.tabAnswer
				clone.tabAnswer = ast.ASTRef{}
			}
		}
		if e.backend == "bmc" || e.backend == "qbmc" {
			// Source statistics belong to the solver that attempted the query,
			// including work completed before cancellation or another failure.
			e.bmcStatistics.Reset()
			e.bmcStatistics.Copy(clone.bmcStatistics)
			if resultErr == nil {
				if err := e.clearBMCTrace(); err != nil {
					resultErr = errors.Join(resultErr, err)
				} else {
					e.bmcTrace = clone.bmcTrace
					clone.bmcTrace = nil
					e.bmcProof = clone.bmcProof
					clone.bmcProof = ast.ASTRef{}
				}
			}
		}
		if e.backend == "ddnf" && e.ddnf.registry == nil && clone.ddnf.registry != nil {
			e.ddnf.registry = clone.ddnf.registry
			clone.ddnf.registry = nil
			clone.ddnf.registryBorrowed = false
		}
		if e.backend == "ddnf" {
			clone.ddnf.transferTrail(&e.ddnf)
		}
		if resultErr == nil {
			e.queryClone = clone
			e.queryCloneRevision = e.revision
		} else {
			if prequeryPDR != nil && clone.pdr == prequeryPDR {
				initialized, stateErr := prequeryPDR.Initialized()
				resultErr = errors.Join(resultErr, stateErr)
				if stateErr == nil && !initialized {
					e.pdr = clone.pdr
					e.pdrRevision = e.revision
					clone.pdr = nil
					clone.pdrRevision = 0
				}
			}
			if prequerySpacer != nil && clone.spacer == prequerySpacer {
				initialized, stateErr := prequerySpacer.Initialized()
				resultErr = errors.Join(resultErr, stateErr)
				if stateErr == nil && !initialized {
					e.spacer = clone.spacer
					e.spacerRevision = e.revision
					clone.spacer = nil
					clone.spacerRevision = 0
				}
			}
			if clone.backend == "spacer" && clone.spacer != nil && clone.spacerRules != nil {
				e.queryClone = clone
				e.queryCloneRevision = e.revision
				return
			}
			if (clone.backend == "pdr" || clone.backend == "qpdr") && clone.pdr != nil && clone.pdrRules != nil {
				// Installed frames remain inspectable after a failed solve;
				// result access still requires the separate successful snapshot.
				e.queryClone = clone
				e.queryCloneRevision = e.revision
				return
			}
			resultErr = errors.Join(resultErr, e.discardQueryClone(clone))
		}
	}()
	if clone.backend == "tab" {
		// TAB initializes its false-head goal from the expression query rule;
		// only the original program belongs in the search rule index.
		return clone.queryTabExpression(query)
	}
	declaration, headRef, err := clone.makeQueryHead(query)
	if err != nil {
		return lbool.Undef, err
	}
	queryRoots := []ast.ASTRef{headRef}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(e.manager, queryRoots)) }()
	head, err := e.manager.RefGet(headRef)
	if err != nil {
		return lbool.Undef, err
	}
	if err = clone.RegisterPredicate(declaration); err != nil {
		return lbool.Undef, err
	}
	clone.auxiliary[declaration] = true
	if clone.backend == "datalog" {
		if err = clone.inheritDatalogQueryKind(query, declaration); err != nil {
			return lbool.Undef, err
		}
	}
	implicationRef, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return lbool.Undef, err
	}
	queryRoots = append(queryRoots, implicationRef)
	rule, err := clone.manager.MkImplies(query, head)
	if !rule.IsNull() {
		err = errors.Join(err, e.manager.SetRef(implicationRef, rule))
	}
	if err != nil {
		return lbool.Undef, err
	}
	if err = clone.AddRule(rule, ""); err != nil {
		return lbool.Undef, err
	}
	karrSelected := false
	relationExplanationSelected := false
	if clone.backend == "datalog" {
		relationExplanationSelected, err = clone.relationLevelNumericExplanationSelected()
		if err != nil {
			return lbool.Undef, err
		}
		if !relationExplanationSelected {
			karrSelected, err = clone.ensureKarrDatalog()
		}
		if err != nil {
			return lbool.Undef, err
		}
	}
	boundIntervalSelected := false
	if clone.backend == "datalog" && !karrSelected && !relationExplanationSelected {
		boundIntervalSelected, err = clone.ensureBoundIntervalDatalog()
		if err != nil {
			return lbool.Undef, err
		}
	}
	if clone.backend == "datalog" && !karrSelected && !boundIntervalSelected && !relationExplanationSelected {
		if _, cause := clone.ensureUDocDatalog(); cause != nil {
			return lbool.Undef, cause
		}
		if selected, cause := clone.ensureSparseIntervalDatalog(); cause != nil {
			return lbool.Undef, cause
		} else if !selected {
			if _, err = clone.ensureIntervalDatalog(); err != nil {
				return lbool.Undef, err
			}
			if _, err = clone.ensureBoundDatalog(); err != nil {
				return lbool.Undef, err
			}
		}
	}
	clone.datalogExplanationQuery, err = clone.datalogExplanationsEnabled()
	if err != nil {
		return lbool.Undef, err
	}
	if !clone.datalogExplanationQuery {
		if err = clone.applyQueryMagicSets(declaration); err != nil {
			return lbool.Undef, err
		}
	}
	// The source PDR context already exists when covers are supplied before
	// the first query. Move that owner after query preparation succeeds.
	if (e.backend == "pdr" || e.backend == "qpdr") && e.pdr != nil && e.pdrRules == nil {
		prequeryPDR = e.pdr
		clone.pdr = e.pdr
		clone.pdrRevision = clone.revision
		e.pdr = nil
		e.pdrRevision = 0
	} else if (e.backend == "pdr" || e.backend == "qpdr") && e.pdr != nil {
		if err := e.closePDRForReplacement(); err != nil {
			return lbool.Undef, err
		}
	}
	// Spacer properties likewise belong to the context that installs the
	// expression query's transformed rules.
	if e.backend == "spacer" && e.spacer != nil && e.spacerRules == nil {
		prequerySpacer = e.spacer
		clone.spacer = e.spacer
		clone.spacerRevision = clone.revision
		e.spacer = nil
		e.spacerRevision = 0
	} else if e.backend == "spacer" && e.spacer != nil {
		if err := e.closeSpacerForReplacement(); err != nil {
			return lbool.Undef, err
		}
	}
	if originalTraceQuery != (ast.ASTRef{}) {
		clone.pdrTraceQuery, err = e.manager.RefGet(originalTraceQuery)
		if err != nil {
			return lbool.Undef, err
		}
		defer func() { clone.pdrTraceQuery = ast.ASTHandle{} }()
	}
	if level != nil {
		result, err = clone.QueryPredicateFromLevel(declaration, *level)
	} else if clone.backend == "ddnf" {
		// DDNF calls mk_query before compiling; its output marker lets the
		// inner context's display_smt2 emit the query instead of its helper.
		if err = clone.set.SetOutputPredicate(declaration); err != nil {
			return lbool.Undef, err
		}
		result, err = clone.queryDDNF()
	} else {
		result, err = clone.QueryPredicate(declaration)
	}
	if err == nil && e.backend == "datalog" {
		err = e.publishDatalogQueryRelations(clone)
	}
	if err == nil {
		e.queryClonePredicate = declaration
	}
	return result, err
}

// clearQueryResources releases the backend state used to answer the previous
// expression query. It is also called when the engine itself is closed.
func (e *PropositionalEngine) clearQueryResources() error {
	// Finish an earlier direct-context replacement before a new expression
	// query can transfer its pre-query owner into a clone.
	if e.pdrDisposal != nil {
		if err := e.closePDRForReplacement(); err != nil {
			return err
		}
	}
	if e.spacerDisposal != nil {
		if err := e.closeSpacerForReplacement(); err != nil {
			return err
		}
	}
	if err := e.clearDatalogTransformCleanup(); err != nil {
		return err
	}
	if err := closeHornModelConversions(e.datalogConversions); err != nil {
		return err
	}
	e.datalogConversions = nil
	if err := e.clearArithmeticTableFactCleanup(); err != nil {
		return err
	}
	if err := e.clearDatalogExplanationCleanup(); err != nil {
		return err
	}
	if e.datalogExplanationAnswer != (ast.ASTRef{}) {
		if err := e.manager.DelRef(&e.datalogExplanationAnswer); err != nil {
			return err
		}
	}
	if err := e.clearQueryCleanup(); err != nil {
		return err
	}
	e.queryRelations = nil
	e.queryRelationsSet = false
	if e.queryClone == nil {
		return nil
	}
	// Destruction can close the solver before a later reference release fails.
	// Save its complete source counters once, while keeping the active clone
	// available for a refused disposal to resume through the same owner.
	if e.queryCloneDisposal == nil {
		e.queryCloneDisposal = &queryCloneCleanup{clone: e.queryClone}
	}
	pending := e.queryCloneDisposal
	if err := pending.captureStatistics(); err != nil {
		return err
	}
	if err := e.queryClone.Close(); err != nil {
		return fmt.Errorf("muz/fp: close expression-query clone: %w", err)
	}
	switch pending.clone.backend {
	case "pdr", "qpdr":
		e.pdrStatistics.Merge(pending.pdr)
	case "spacer":
		e.spacerStatistics.Merge(pending.spacer)
	}
	e.queryClone = nil
	e.queryCloneDisposal = nil
	e.queryCloneRevision = 0
	e.queryClonePredicate = ast.ASTHandle{}
	return nil
}

// ExpressionAnswer returns the backend answer to the most recent expression
// query while its query snapshot is valid. The caller owns the returned ref.
func (e *PropositionalEngine) expressionQueryState() (*PropositionalEngine, EngineKind, error) {
	if err := e.open(); err != nil {
		return nil, EngineDatalog, err
	}
	kind, err := parseEngineKind(e.backend)
	if err != nil || !e.queryReady || e.queryRevision != e.revision || e.queryEngine != kind || e.queryClone == nil {
		return nil, kind, ErrPropositionalEngine
	}
	return e.queryClone, kind, nil
}

func (e *PropositionalEngine) ExpressionAnswer() (ast.ASTRef, error) {
	clone, kind, err := e.expressionQueryState()
	if err != nil {
		return ast.ASTRef{}, err
	}
	// BMC and tabulation transfer their proof/answer to the parent during query
	// cleanup; their answer methods do not inspect the predicate argument.
	if kind == EngineBMC || kind == EngineQBMC || kind == EngineTab {
		return e.Answer(e.queryClonePredicate)
	}
	return clone.Answer(e.queryClonePredicate)
}

// ExpressionGroundSatAnswer returns the Spacer ground answer for the latest
// expression query. The caller owns the returned reference.
func (e *PropositionalEngine) ExpressionGroundSatAnswer() (ast.ASTRef, error) {
	clone, kind, err := e.expressionQueryState()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if kind != EngineSpacer {
		return ast.ASTRef{}, ErrPropositionalEngine
	}
	return clone.GroundSatAnswer(e.queryClonePredicate)
}
