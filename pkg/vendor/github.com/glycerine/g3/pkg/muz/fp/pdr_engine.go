package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp query setup and PDR dispatch.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type pdrContextDisposal struct {
	context    *pdr.Context
	statistics pdr.StatisticsSnapshot
}

func (e *PropositionalEngine) snapshotPDRStatistics() (pdr.StatisticsSnapshot, error) {
	var result pdr.StatisticsSnapshot
	result.Merge(e.pdrStatistics)
	for _, pending := range e.queryCleanup {
		if pending.clone.backend == "pdr" || pending.clone.backend == "qpdr" {
			if err := pending.captureStatistics(); err != nil {
				return result, err
			}
			result.Merge(pending.pdr)
		}
	}
	if e.pdr != nil {
		if pending := e.pdrDisposal; pending != nil {
			result.Merge(pending.statistics)
		} else {
			current, err := e.pdr.SnapshotStatistics()
			if err != nil {
				return result, err
			}
			result.Merge(current)
		}
	}
	if e.queryClone != nil && (e.queryClone.backend == "pdr" || e.queryClone.backend == "qpdr") {
		if pending := e.queryCloneDisposal; pending != nil && pending.statisticsReady && pending.clone.closing {
			result.Merge(pending.pdr)
		} else {
			current, err := e.queryClone.snapshotPDRStatistics()
			if err != nil {
				return result, err
			}
			result.Merge(current)
		}
	}
	return result, nil
}

func (e *PropositionalEngine) collectPDRStatistics(destination *statistics.Statistics, previous pdr.StatisticsSnapshot) error {
	previous.Merge(e.pdrStatistics)
	if pending := e.pdrDisposal; pending != nil {
		previous.Merge(pending.statistics)
	}
	for _, pending := range e.queryCleanup {
		if pending.clone.backend == "pdr" || pending.clone.backend == "qpdr" {
			if err := pending.captureStatistics(); err != nil {
				return err
			}
			previous.Merge(pending.pdr)
		}
	}
	if e.queryClone != nil && (e.queryClone.backend == "pdr" || e.queryClone.backend == "qpdr") {
		if pending := e.queryCloneDisposal; pending != nil && pending.statisticsReady && pending.clone.closing {
			previous.Merge(pending.pdr)
		} else {
			if e.pdr != nil && e.pdrDisposal == nil {
				current, err := e.pdr.SnapshotStatistics()
				if err != nil {
					return err
				}
				previous.Merge(current)
			}
			return e.queryClone.collectPDRStatistics(destination, previous)
		}
	}
	if e.pdr != nil && e.pdrDisposal == nil {
		return e.pdr.CollectStatisticsWithSnapshot(destination, previous)
	}
	return previous.Collect(destination)
}

func (e *PropositionalEngine) closePDRForReplacement() error {
	if e.pdr != nil {
		// A later source member can refuse destruction after the solver pool
		// has closed. Keep its complete snapshot and resume that exact owner
		// without asking the closing manager to collect statistics again.
		if e.pdrDisposal == nil {
			archived, err := e.pdr.SnapshotStatistics()
			if err != nil {
				return err
			}
			e.pdrDisposal = &pdrContextDisposal{context: e.pdr, statistics: archived}
		}
		pending := e.pdrDisposal
		if err := pending.context.Close(); err != nil {
			return err
		}
		e.pdrStatistics.Merge(pending.statistics)
		e.pdr = nil
		e.pdrDisposal = nil
	}
	if e.pdrRules != nil {
		if err := e.pdrRules.Close(); err != nil {
			return err
		}
		e.pdrRules = nil
	}
	if e.pdrSourceRules != nil {
		if err := e.pdrSourceRules.Close(); err != nil {
			return err
		}
		e.pdrSourceRules = nil
	}
	if err := e.clearPDRPreprocessing(); err != nil {
		return err
	}
	e.pdrRevision = 0
	e.pdrRuleRevision = 0
	e.pdrQuery = ast.ASTHandle{}
	return nil
}

func (e *PropositionalEngine) pdrPredicate(declaration ast.ASTHandle) ast.ASTHandle {
	if derived, found := e.pdrDerived[declaration]; found {
		return derived
	}
	return hornDerivedPredicate(declaration, e.pdrQuantified, e.pdrScaled, e.pdrMagic, e.pdrBitBlasted, e.pdrInstantiated, e.pdrPredicates)
}

func (e *PropositionalEngine) clearPDRPreprocessing() error {
	if err := closeHornModelConversions(e.pdrConversions); err != nil {
		return err
	}
	e.pdrConversions = nil
	if e.pdrProof != (ast.ASTRef{}) {
		if err := e.manager.DelRef(&e.pdrProof); err != nil {
			return err
		}
	}
	e.pdrSliceProof = false
	e.pdrDerived = nil
	e.pdrPredicates = nil
	e.pdrSlices = nil
	e.pdrQuantified = nil
	e.pdrInstantiated = nil
	e.pdrBitBlasted = nil
	e.pdrScaled = nil
	e.pdrMagic = nil
	e.pdrKarr = nil
	e.pdrConversions = nil
	return nil
}

func (e *PropositionalEngine) installPDRPreprocessing(result *hornPreprocessResult) {
	e.pdrPredicates = result.slicedPredicates
	e.pdrSlices = result.slicedColumns
	e.pdrQuantified = result.quantifiedPredicates
	e.pdrInstantiated = result.instantiatedPredicates
	e.pdrBitBlasted = result.bitBlastedPredicates
	e.pdrScaled = result.scaledPredicates
	e.pdrMagic = result.magicPredicates
	e.pdrKarr = result.karrInvariants
	e.pdrConversions = result.takeModelConversions()
}

// The source interface marks an empty transformed program unsatisfiable
// directly. It does not enter context::solve (and therefore emits none of the
// solve TRACE/validation/cancellation lifecycle) before printing its model.
func solvePDRProgram(context *pdr.Context, rules *base.RuleSet, query ast.ASTHandle) (lbool.LBool, error) {
	count, err := rules.Len()
	if err != nil {
		return lbool.Undef, err
	}
	if count != 0 {
		return context.Solve(query)
	}
	if err = context.SetEmptyProgramUnsat(query); err != nil {
		return lbool.Undef, err
	}
	return lbool.False, nil
}

func (e *PropositionalEngine) queryPDR(declaration ast.ASTHandle) (_ lbool.LBool, resultErr error) {
	status, err := e.queryPDRWithSliceProof(declaration, true)
	if onlyErrorKind(err, errRetryUnslicedProof) {
		if closeErr := e.closePDRForReplacement(); closeErr != nil {
			return lbool.Undef, closeErr
		}
		return e.queryPDRWithSliceProof(declaration, false)
	}
	return status, err
}

func (e *PropositionalEngine) queryPDRWithSliceProof(declaration ast.ASTHandle, allowSliceProof bool) (_ lbool.LBool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, fmt.Errorf("muz/fp: PDR requires parameter and global services")
	}
	if e.spacer != nil {
		if err := e.closeSpacerForReplacement(); err != nil {
			return lbool.Undef, err
		}
		e.spacerPredicates = nil
		e.spacerSlices = nil
		e.spacerQuantified = nil
		e.spacerInstantiated = nil
		e.spacerBitBlasted = nil
		e.spacerScaled = nil
		e.spacerMagic = nil
		e.spacerKarr = nil
	}
	if err := closeHornModelConversions(e.spacerConversions); err != nil {
		return lbool.Undef, errors.Join(resultErr, err)
	}
	e.spacerConversions = nil
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, x := e.set.Finalize()
		if x != nil {
			return lbool.Undef, x
		}
		if !valid {
			return lbool.Undef, ErrUnstratifiedNegation
		}
	}
	// Match pdr_dl_interface's default Horn transformations. Relational
	// filter/simple-join normalization belongs to the Datalog query path.
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	// Source check_reset scans original rules before default transformations.
	// rule_manager::mk_query then contributes a fresh synthetic query head, so
	// every query after the first resets the installed context even when the
	// caller supplies the same declaration and persistent program.
	if err = e.checkPDRSourceRules(); err != nil {
		return lbool.Undef, err
	}
	if err = e.tracePDRInput(declaration); err != nil {
		return lbool.Undef, err
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	context, err := e.newConfiguredPDRContext(parameters)
	if err != nil {
		return lbool.Undef, err
	}
	closeContext := func() error {
		if context == e.pdr {
			if e.pdrRules == nil {
				initialized, err := context.Initialized()
				if err == nil && !initialized {
					return nil
				}
				if err != nil {
					return errors.Join(err, e.closePDRForReplacement())
				}
			}
			return e.closePDRForReplacement()
		}
		return context.Close()
	}
	preprocessed, err := e.preprocessHornRulesWithSliceProof(declaration, true, true, allowSliceProof)
	if err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	defer func() { resultErr = errors.Join(resultErr, preprocessed.Close()) }()
	prepared := preprocessed.rules
	if err = e.tracePDRTransformedRules(prepared); err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	if err = e.writePDRQueryRules(prepared); err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	snapshot, err := snapshotHornRules(prepared)
	if err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	sourceSnapshot, err := snapshotHornRules(e.set)
	if err != nil {
		return lbool.Undef, errors.Join(err, snapshot.Close(), closeContext())
	}
	keepSnapshot := false
	defer func() {
		if !keepSnapshot {
			resultErr = errors.Join(resultErr, snapshot.Close(), sourceSnapshot.Close())
		}
	}()
	installed := false
	defer func() {
		if !installed {
			resultErr = errors.Join(resultErr, closeHornModelConversions(preprocessed.modelConversions))
			preprocessed.modelConversions = nil
		}
	}()
	derivedPredicates := make(map[ast.ASTHandle]ast.ASTHandle, len(e.predicateOrder)+len(e.auxiliary)+1)
	for _, original := range e.predicateOrder {
		derived, deriveErr := prepared.DerivedPredicate(original)
		if deriveErr != nil {
			return lbool.Undef, errors.Join(fmt.Errorf("muz/fp: resolve preprocessed PDR predicate: %w", deriveErr), context.Close())
		}
		derivedPredicates[original] = derived
	}
	queryDeclaration, err := prepared.DerivedPredicate(declaration)
	if err != nil {
		return lbool.Undef, errors.Join(fmt.Errorf("muz/fp: resolve preprocessed PDR query: %w", err), context.Close())
	}
	auxiliary := make([]ast.ASTHandle, 0, len(e.auxiliary))
	for declaration := range e.auxiliary {
		derived := hornDerivedPredicate(declaration, preprocessed.quantifiedPredicates, preprocessed.scaledPredicates, preprocessed.magicPredicates, preprocessed.bitBlastedPredicates, preprocessed.instantiatedPredicates, preprocessed.slicedPredicates)
		auxiliary = append(auxiliary, derived)
	}
	reset := e.pdr != nil && e.pdrRules != nil && e.pdrRuleRevision == e.revision
	if e.pdr != nil && e.pdrRules == nil {
		// Source check_reset preserves covers supplied before any old rules.
		if err = context.Close(); err != nil {
			return lbool.Undef, err
		}
		context = e.pdr
	} else if reset {
		if err = context.Close(); err != nil {
			return lbool.Undef, err
		}
		context = e.pdr
		if err = context.Reset(); err != nil {
			return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
		}
		if err = e.clearPDRPreprocessing(); err != nil {
			return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
		}
	} else if err = e.closePDRForReplacement(); err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	if err = context.SetAuxiliaryPredicates(auxiliary); err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	background, backgroundErr := e.manager.RefGet(e.background)
	if backgroundErr != nil {
		return lbool.Undef, errors.Join(backgroundErr, closeContext())
	}
	if err = context.SetBackground(background); err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	if err = context.Initialize(prepared); err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	if reset {
		if err = e.replacePDRRuleSnapshots(snapshot, sourceSnapshot); err != nil {
			return lbool.Undef, errors.Join(err, e.closePDRForReplacement())
		}
	} else {
		e.pdrRules = snapshot
		e.pdrSourceRules = sourceSnapshot
	}
	e.installPDRPreprocessing(preprocessed)
	e.pdrSliceProof = preprocessed.sliceProof != nil
	e.pdrDerived = derivedPredicates
	e.pdr = context
	e.pdrRevision = e.revision
	e.pdrRuleRevision = e.revision
	e.pdrQuery = declaration
	keepSnapshot = true
	installed = true
	status, err := solvePDRProgram(context, e.pdrRules, queryDeclaration)
	if err == nil && status == lbool.False {
		err = e.writePDREmptyProgramModel(declaration)
	}
	if err != nil || status != lbool.True || preprocessed.sliceProof == nil {
		return status, err
	}
	proof, err := context.Proof(queryDeclaration)
	if err != nil {
		if onlyErrorKind(err, ast.ErrASTDomain) {
			return lbool.Undef, fmt.Errorf("%w: context proof: %v", errRetryUnslicedProof, err)
		}
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&proof)) }()
	original, err := e.manager.RefGet(proof)
	if err != nil {
		return lbool.Undef, err
	}
	converted, err := preprocessed.sliceProof.Convert(original)
	if err != nil {
		if onlyErrorKind(err, transforms.ErrSliceProofUnsupported) || onlyErrorKind(err, ast.ErrASTDomain) {
			return lbool.Undef, fmt.Errorf("%w: %v", errRetryUnslicedProof, err)
		}
		return lbool.Undef, err
	}
	convertedHandle, err := e.manager.RefGet(converted)
	if err == nil {
		var before, after ast.ASTHandle
		before, err = e.manager.GetProofFact(original)
		if err == nil {
			after, err = e.manager.GetProofFact(convertedHandle)
		}
		if err == nil && before != after {
			err = errRetryUnslicedProof
		}
	}
	if err != nil {
		return lbool.Undef, errors.Join(err, e.manager.DelRef(&converted))
	}
	e.pdrProof = converted
	return status, nil
}

func (e *PropositionalEngine) pdrContext() (_ *pdr.Context, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.backend != "pdr" && e.backend != "qpdr" {
		return nil, ErrPropositionalEngine
	}
	if e.pdrDisposal != nil {
		return nil, ast.ErrASTPending
	}
	if e.pdr != nil {
		if e.pdrRules != nil && e.pdrRevision != e.revision {
			return nil, ErrPropositionalEngine
		}
		return e.pdr, nil
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	context, err := e.newConfiguredPDRContext(parameters)
	if err != nil {
		return nil, err
	}
	background, err := e.manager.RefGet(e.background)
	if err == nil {
		err = context.SetBackground(background)
	}
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	e.pdr = context
	e.pdrRevision = e.revision
	return context, nil
}

// Source level/cover access remains valid after a failed solve. Its program
// revision is independent of the successful-result snapshot used by answers.
func (e *PropositionalEngine) propertyQueryClone() (*PropositionalEngine, error) {
	if e.queryClone == nil || e.queryCloneRevision != e.revision || e.queryClone.backend != e.backend {
		return nil, ErrPropositionalEngine
	}
	return e.queryClone, nil
}

// NumLevels returns the finite frame count for a PDR or Spacer relation.
func (e *PropositionalEngine) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	if err := e.open(); err != nil {
		return 0, err
	}
	// Source context property entry points resolve the configured engine.
	if err := e.selectFactEngine(); err != nil {
		return 0, err
	}
	// Duality's engine_base implementation asserts for level queries; it has
	// no PDR frames. Do not let the shared PDR fallback create a PDR context.
	if e.backend == "duality" {
		return 0, ErrPropositionalEngine
	}
	if e.queryClone != nil && (e.backend == "pdr" || e.backend == "qpdr" || e.backend == "spacer") {
		clone, err := e.propertyQueryClone()
		if err != nil {
			return 0, err
		}
		return clone.NumLevels(declaration)
	}
	if e.backend == "spacer" {
		context, err := e.spacerContext()
		if err != nil {
			return 0, err
		}

		return context.NumLevels(e.spacerPredicate(declaration))
	}
	context, err := e.pdrContext()
	if err != nil {
		return 0, err
	}
	return context.NumLevels(e.pdrPredicate(declaration))
}

// CoverDelta returns an owning reference to a PDR or Spacer cover frame.
func (e *PropositionalEngine) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	// Source context property entry points resolve the configured engine.
	if err := e.selectFactEngine(); err != nil {
		return ast.ASTRef{}, err
	}
	// The Duality implementation's get_cover_delta is an assertion stub.
	if e.backend == "duality" {
		return ast.ASTRef{}, ErrPropositionalEngine
	}
	if e.queryClone != nil && (e.backend == "pdr" || e.backend == "qpdr" || e.backend == "spacer") {
		clone, err := e.propertyQueryClone()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return clone.CoverDelta(level, declaration)
	}
	if e.backend == "spacer" {
		context, err := e.spacerContext()
		if err != nil {
			return ast.ASTRef{}, err
		}

		derived := e.spacerPredicate(declaration)
		_, found, err := context.Transformer(derived)
		if err != nil {
			return ast.ASTRef{}, err
		}
		cover, err := context.CoverDelta(level, derived)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if !found {
			return cover, nil
		}
		return e.expandHornCover(declaration, derived, cover, true, e.spacerConversions)
	}
	context, err := e.pdrContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	derived := e.pdrPredicate(declaration)
	if e.pdrRules == nil {
		return context.CoverDelta(level, derived)
	}
	_, found, err := context.Transformer(derived)
	if err != nil {
		return ast.ASTRef{}, err
	}
	cover, err := context.CoverDelta(level, derived)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !found {
		return cover, nil
	}
	return e.expandHornCover(declaration, derived, cover, true, e.pdrConversions)
}

// AddCover installs a bound-variable property in a retained PDR or Spacer
// frame.
func (e *PropositionalEngine) AddCover(level int, declaration, property ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	// Source context property entry points resolve the configured engine.
	if err := e.selectFactEngine(); err != nil {
		return err
	}
	// The Duality implementation's add_cover is an assertion stub.
	if e.backend == "duality" {
		return ErrPropositionalEngine
	}
	if e.queryClone != nil && (e.backend == "pdr" || e.backend == "qpdr" || e.backend == "spacer") {
		clone, err := e.propertyQueryClone()
		if err != nil {
			return err
		}
		return clone.AddCover(level, declaration, property)
	}
	if e.backend == "spacer" {
		context, err := e.spacerContext()
		if err != nil {
			return err
		}

		if err := e.rejectSlicedProperty(declaration); err != nil {
			return err
		}
		eliminated, err := e.hornInlineEliminated(e.spacerConversions, declaration)
		if err != nil {
			return err
		}
		if eliminated {
			return fmt.Errorf("muz/fp: covers and invariants are incompatible with eliminated predicates")
		}
		return context.AddCover(level, e.spacerPredicate(declaration), property)
	}
	context, err := e.pdrContext()
	if err != nil {
		return err
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return err
	}
	slicing, err := parameters.Bool("xform.slice", true)
	err = errors.Join(err, parameters.Close())
	if err != nil {
		return err
	}
	if slicing {
		return fmt.Errorf("Covers are incompatible with slicing. Disable slicing before using covers")
	}
	if slicedPredicateHasRemovedColumns(e.pdrSlices, declaration) {
		return fmt.Errorf("muz/fp: covers and invariants are incompatible with sliced predicate columns")
	}
	eliminated, err := e.hornInlineEliminated(e.pdrConversions, declaration)
	if err != nil {
		return err
	}
	if eliminated {
		return fmt.Errorf("muz/fp: covers and invariants are incompatible with eliminated predicates")
	}
	return context.AddCover(level, e.pdrPredicate(declaration), property)
}

// AddInvariant installs a bound-variable property in the invariant frame.
func (e *PropositionalEngine) AddInvariant(declaration, property ast.ASTHandle) error {
	return e.AddCover(-1, declaration, property)
}

// Reachable returns the accumulated Spacer reach summary for a predicate.
func (e *PropositionalEngine) Reachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if e.queryClone != nil && e.backend == "spacer" {
		clone, _, err := e.expressionQueryState()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return clone.Reachable(declaration)
	}
	if err := e.requireResultSnapshot(EngineSpacer); err != nil {
		return ast.ASTRef{}, err
	}
	derived := e.spacerPredicate(declaration)
	_, found, err := e.spacer.Transformer(derived)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, err := e.spacer.Reachable(derived)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return e.expandHornCover(declaration, derived, result, found, e.spacerConversions)
}

// GroundSatAnswer returns the latest successful Spacer reach-fact answer.
func (e *PropositionalEngine) GroundSatAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.requireResultSnapshot(EngineSpacer); err != nil {
		return ast.ASTRef{}, err
	}
	if declaration != e.spacerOriginalQuery {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	answer, err := e.spacer.GroundSatAnswer(e.spacerQuery)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if answer == (ast.ASTRef{}) {
		return answer, nil
	}
	return e.expandSpacerFormula(declaration, answer)
}

// RulesAlongTrace returns owning rule formulas and the source symbol::str
// spelling of each name, including "<null>" for an absent label.
func (e *PropositionalEngine) RulesAlongTrace() (formulas []ast.ASTRef, names []string, resultErr error) {
	formulas, labels, err := e.RulesAlongTraceWithSymbols()
	if err != nil {
		return nil, nil, err
	}
	names = make([]string, len(labels))
	for i, label := range labels {
		names[i], err = e.manager.Symbols.String(label)
		if err != nil {
			a := &contextFormulaAttempt{manager: e.manager, references: formulas}
			return nil, nil, a.finish(err)
		}
	}
	return formulas, names, nil
}

// RulesAlongTraceWithSymbols preserves the source trace label identities and
// returns caller-owned rule formulas for the latest Spacer or BMC query.
func (e *PropositionalEngine) RulesAlongTraceWithSymbols() (formulas []ast.ASTRef, names []symbol.Symbol, resultErr error) {
	if e.queryClone != nil && e.backend == "spacer" {
		clone, _, err := e.expressionQueryState()
		if err != nil {
			return nil, nil, err
		}
		return clone.RulesAlongTraceWithSymbols()
	}
	if e.backend == "bmc" || e.backend == "qbmc" {
		return e.bmcRulesAlongTrace()
	}
	if err := e.requireResultSnapshot(EngineSpacer); err != nil {
		return nil, nil, err
	}
	rules, err := e.spacer.RulesAlongTrace()
	a := &contextFormulaAttempt{manager: e.manager, rules: rules}
	if err != nil {
		return nil, nil, a.finish(err)
	}
	defer func() {
		if resultErr != nil {
			a.references = append(a.references, formulas...)
			formulas = nil
			names = nil
		}
		resultErr = a.finish(resultErr)
		if resultErr != nil && formulas != nil {
			a.references = append(a.references, formulas...)
			formulas = nil
			names = nil
		}
	}()
	for _, rule := range rules {
		formula, err := rule.ToFormula()
		if formula != (ast.ASTRef{}) {
			formulas = append(formulas, formula)
		}
		if err != nil {
			return formulas, names, err
		}
		name, err := rule.NameSymbol()
		if err != nil {
			return formulas, names, err
		}
		names = append(names, name)
	}
	return formulas, names, resultErr
}

// newConfiguredPDRContext owns construction until every source parameter is installed.
func (e *PropositionalEngine) newConfiguredPDRContext(parameters *fixedpointParamView) (*pdr.Context, error) {
	maximumContexts, err := parameters.UInt("pdr.max_num_contexts", 500)
	if err != nil {
		return nil, err
	}
	cacheValue, err := parameters.UInt("pdr.cache_mode", uint32(pdr.ReachableNoCache))
	if err != nil {
		return nil, err
	}
	cacheMode := pdr.ReachableCacheMode(cacheValue)
	if cacheMode > pdr.ReachableConstraintCache {
		return nil, fmt.Errorf("muz/fp: invalid pdr.cache_mode %d", cacheValue)
	}
	theory := e.pdrTheory
	if theory == nil {
		theory = e.guard
	}
	context, err := pdr.NewContextWithTheory(e.manager, parameters.local, maximumContexts, cacheMode, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}, theory)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetCertificateModelConverter(pdrCertificateModelConverter{engine: e}); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	simplifyPre, err := parameters.Bool("pdr.simplify_formulas_pre", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	simplifyPost, err := parameters.Bool("pdr.simplify_formulas_post", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetFrameSimplification(simplifyPre, simplifyPost); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	validateResult, err := parameters.Bool("pdr.validate_result", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetResultValidation(validateResult); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useFarkas, err := parameters.Bool("pdr.farkas", true)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseFarkas(useFarkas); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useInductive, err := parameters.Bool("pdr.use_inductive_generalizer", true)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseInductiveGeneralizer(useInductive); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useMultiCore, err := parameters.Bool("pdr.use_multicore_generalizer", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseMultiCoreGeneralizer(useMultiCore); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useArithmeticInductive, err := parameters.Bool("pdr.use_arith_inductive_generalizer", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseArithmeticInductiveGeneralizer(useArithmeticInductive); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useInductiveReachability, err := parameters.Bool("pdr.inductive_reachability_check", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseInductiveReachabilityGeneralizer(useInductiveReachability); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useConvexClosure, err := parameters.Bool("pdr.use_convex_closure_generalizer", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseConvexClosureGeneralizer(useConvexClosure); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useConvexInterior, err := parameters.Bool("pdr.use_convex_interior_generalizer", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseConvexInteriorGeneralizer(useConvexInterior); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useUTVPI, err := parameters.Bool("pdr.utvpi", true)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseUTVPI(useUTVPI); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	useModelGeneralizer, err := parameters.Bool("pdr.use_model_generalizer", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetUseModelGeneralizer(useModelGeneralizer); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	breadthFirst, err := parameters.Bool("pdr.bfs_model_search", true)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetBreadthFirstModelSearch(breadthFirst); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	flexible, err := parameters.Bool("pdr.flexible_trace", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetFlexibleTrace(flexible); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	inlineEager, err := parameters.Bool("xform.inline_eager", true)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	inlineLinear, err := parameters.Bool("xform.inline_linear", true)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetFullTraceAnswer(!inlineEager && !inlineLinear); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	generateProofTrace, err := parameters.Bool("generate_proof_trace", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	printBoogieCertificate, err := parameters.Bool("print_boogie_certificate", false)
	if err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetPrintBoogieCertificate(printBoogieCertificate); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	if err = context.SetGenerateProofTrace(generateProofTrace || printBoogieCertificate || e.forceProofTrace); err != nil {
		return failPDRContextConstruction(context, e.manager.NumeralService().RuntimeOwner(), err)
	}
	return context, nil
}
