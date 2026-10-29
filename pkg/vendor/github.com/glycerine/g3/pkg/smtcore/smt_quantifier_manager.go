package smtcore

// Source: src/smt/smt_quantifier.cpp quantifier_manager propagation, queue
// instantiation, quick checking, final-check arbitration, and scope lifecycle.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// SMTQuantifierManager composes the concrete quantifier components owned by
// the Go SMT context. Pattern matching and model-based checking add instances
// through Registry; this manager schedules and internalizes those instances.
type SMTQuantifierManager struct {
	state                       *SMTInternalizerState
	registry                    *SMTQuantifierRegistry
	rewriter                    *rewriter.TheoryRewriter
	checker                     *SMTChecker
	quick                       *SMTQuickChecker
	substitute                  *SMTCachedVarSubstitution
	builder                     *SMTQIInstanceBuilder
	mam                         *SMTMAM
	lazyMAM                     *SMTMAM
	modelFinder                 *SMTModelFinder
	modelChecker                *SMTModelChecker
	auxiliarySolver             smtAuxiliarySolverFactory
	candidateModel              func() (*SMTModelGenerator, error)
	acceptedModel               *SMTModelGenerator
	activated                   map[ast.ASTHandle]bool
	activationTrail             []ast.ASTHandle
	activationScopes            []int
	newEnodeHead                int
	newEnodeScopes              []int
	lazyMatchingIndex           uint32
	lazyMatchingTrail           []uint32
	lazyMatchingScopes          []int
	maxEagerMultipatterns       uint32
	maxLazyMultipatternMatching uint32
	lazyInstantiation           bool
	promoteUnsat                bool
	ematching                   bool
	quickUnsat                  bool
	quickNotSat                 bool
	lazyQuick                   bool
	mbqi                        bool
	mbqiID                      string
	mbqiTrace                   bool
	closed                      bool
}

type SMTQuantifierManagerConfig struct {
	MaxEagerMultipatterns       uint32
	MaxLazyMultipatternMatching uint32
	LazyInstantiation           bool
	Ematching                   bool
	PromoteUnsat                bool
	QuickUnsat                  bool
	QuickNotSat                 bool
	LazyQuick                   bool
	MBQI                        bool
	MaxCexs                     uint32
	MaxIterations               uint32
	ForceTemplate               uint32
	MBQIID                      string
	MBQITrace                   bool
	CandidateModel              func() (*SMTModelGenerator, error)
}

func NewSMTQuantifierManager(state *SMTInternalizerState, rw *rewriter.TheoryRewriter, config SMTQuantifierManagerConfig) (*SMTQuantifierManager, error) {
	if state == nil || state.manager == nil || state.quantifiers == nil || rw == nil || rw.Manager() != state.manager {
		return nil, ast.ErrASTHandle
	}
	if state.quantifierManager != nil {
		return nil, ast.ErrASTDomain
	}
	checker, err := NewSMTChecker(state.context)
	if err != nil {
		return nil, err
	}
	quick, err := NewSMTQuickChecker(state.context, rw, state.quantifiers)
	if err != nil {
		return nil, err
	}
	quick.state = state
	substitute, err := NewSMTCachedVarSubstitution(state.manager)
	if err != nil {
		if substitute != nil {
			err = errors.Join(err, substitute.Close())
		}
		return nil, err
	}
	builder, err := NewSMTQIInstanceBuilder(state.manager, checker, substitute, state.quantifiers.costs)
	if err != nil {
		return nil, errors.Join(err, substitute.Close())
	}
	mam, err := NewSMTMAM(state, state.quantifiers)
	if err != nil {
		return nil, errors.Join(err, substitute.Close())
	}
	lazyMAM, err := NewSMTMAM(state, state.quantifiers)
	if err != nil {
		return nil, errors.Join(err, substitute.Close())
	}
	modelFinder, err := NewSMTModelFinder(state)
	if err != nil {
		return nil, errors.Join(err, substitute.Close())
	}
	modelFinder.SetForceTemplate(config.ForceTemplate)
	result := &SMTQuantifierManager{
		state: state, registry: state.quantifiers, rewriter: rw, checker: checker, quick: quick,
		substitute: substitute, builder: builder, mam: mam, lazyMAM: lazyMAM, modelFinder: modelFinder, activated: make(map[ast.ASTHandle]bool),
		maxEagerMultipatterns:       config.MaxEagerMultipatterns,
		maxLazyMultipatternMatching: config.MaxLazyMultipatternMatching,
		lazyInstantiation:           config.LazyInstantiation,
		promoteUnsat:                config.PromoteUnsat, ematching: config.Ematching,
		quickUnsat: config.QuickUnsat, quickNotSat: config.QuickNotSat, lazyQuick: config.LazyQuick,
		mbqi: config.MBQI, mbqiID: config.MBQIID, mbqiTrace: config.MBQITrace, candidateModel: config.CandidateModel,
	}
	if config.MBQI {
		result.modelChecker, err = NewSMTModelChecker(state, config.MaxCexs)
		if err != nil {
			return nil, errors.Join(err, modelFinder.Close(), substitute.Close())
		}
		result.modelChecker.trace = config.MBQITrace
		if err = result.modelChecker.ConfigureLifecycle(config.MaxCexs, config.MaxIterations); err != nil {
			return nil, errors.Join(err, result.modelChecker.Close(), modelFinder.Close(), substitute.Close())
		}
		if err = result.modelChecker.SetModelFinder(modelFinder); err != nil {
			return nil, errors.Join(err, result.modelChecker.Close(), modelFinder.Close(), substitute.Close())
		}
	}
	state.quantifierManager = result
	return result, nil
}

func (q *SMTQuantifierManager) UpdateMBQI(enabled bool, identifier string, maxCexs, maxIterations, forceTemplate uint32) error {
	if err := q.open(); err != nil {
		return err
	}
	if enabled && q.modelChecker == nil {
		checker, err := NewSMTModelChecker(q.state, maxCexs)
		if err != nil {
			return err
		}
		if err = checker.ConfigureLifecycle(maxCexs, maxIterations); err != nil {
			return errors.Join(err, checker.Close())
		}
		if err = checker.SetModelFinder(q.modelFinder); err != nil {
			return errors.Join(err, checker.Close())
		}
		if err = checker.SetAuxiliarySolverFactory(q.auxiliarySolver); err != nil {
			return errors.Join(err, checker.Close())
		}
		q.modelChecker = checker
	} else if enabled {
		if err := q.modelChecker.ConfigureLifecycle(maxCexs, maxIterations); err != nil {
			return err
		}
	} else if q.modelChecker != nil {
		if err := q.modelChecker.Close(); err != nil {
			return err
		}
		q.modelChecker = nil
	}
	if q.modelChecker != nil {
		q.modelChecker.trace = q.mbqiTrace
	}
	q.mbqi = enabled
	q.mbqiID = identifier
	q.modelFinder.SetForceTemplate(forceTemplate)
	return nil
}

func (q *SMTQuantifierManager) SetAuxiliarySolverFactory(factory smtAuxiliarySolverFactory) error {
	if err := q.open(); err != nil {
		return err
	}
	q.auxiliarySolver = factory
	if q.modelChecker == nil {
		return nil
	}
	return q.modelChecker.SetAuxiliarySolverFactory(factory)
}

func (q *SMTQuantifierManager) open() error {
	if q == nil || q.state == nil || q.registry == nil || q.builder == nil || q.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (q *SMTQuantifierManager) InitSearch() error {
	if err := q.open(); err != nil {
		return err
	}
	q.lazyMatchingIndex = 0
	if q.acceptedModel != nil {
		if err := q.acceptedModel.Close(); err != nil {
			return err
		}
		q.acceptedModel = nil
	}
	if err := q.substitute.Reset(); err != nil {
		return err
	}
	if err := q.modelFinder.InitSearch(); err != nil {
		return err
	}
	if q.modelChecker != nil {
		if err := q.modelChecker.InitSearch(); err != nil {
			return err
		}
	}
	return q.registry.InitSearch()
}

func (q *SMTQuantifierManager) CanPropagate() bool {
	if q == nil || q.closed || q.registry == nil {
		return false
	}
	if q.registry.schedule.HasWork() || q.ematching && !q.registry.Empty() && q.mam.HasWork() {
		return true
	}
	if q.hasNewEnodes() {
		return true
	}
	if q.ematching {
		for _, quantifier := range q.registry.quantifiers {
			if q.active(quantifier) && !q.activated[quantifier] {
				return true
			}
		}
	}
	return false
}

// Source default_qm_plugin::propagate scans newly internalized enodes when
// relevancy is disabled, since no relevant_eh notifications will arrive.
func (q *SMTQuantifierManager) hasNewEnodes() bool {
	return q.ematching && !q.registry.Empty() && q.state.relevancy != nil && !q.state.relevancy.enabled && q.newEnodeHead < len(q.state.enodeStack)
}

func (q *SMTQuantifierManager) notifyNewEnodes() {
	if !q.hasNewEnodes() {
		return
	}
	for _, node := range q.state.enodeStack[q.newEnodeHead:] {
		q.mam.Relevant(node)
		q.lazyMAM.Relevant(node)
	}
	q.newEnodeHead = len(q.state.enodeStack)
}

func (q *SMTQuantifierManager) activatePatterns() error {
	if !q.ematching {
		return nil
	}
	for _, quantifier := range q.registry.quantifiers {
		if !q.active(quantifier) || q.activated[quantifier] {
			continue
		}
		count, err := q.state.manager.GetNumPatterns(quantifier)
		if err != nil {
			return err
		}
		hasUnary := false
		for i := uint32(0); i < count; i++ {
			pattern, err := q.state.manager.GetPattern(quantifier, i)
			if err != nil {
				return err
			}
			terms, err := q.state.manager.GetNumArgs(pattern)
			if err != nil {
				return err
			}
			hasUnary = hasUnary || terms == 1
		}
		eagerLimit := q.maxEagerMultipatterns
		if !hasUnary {
			eagerLimit++
		}
		var multipatterns uint32
		recursive, err := q.state.manager.IsRecursiveFunctionDefinition(quantifier)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			if recursive && i > 0 {
				continue
			}
			pattern, err := q.state.manager.GetPattern(quantifier, i)
			if err != nil {
				return err
			}
			terms, err := q.state.manager.GetNumArgs(pattern)
			if err != nil {
				return err
			}
			matcher := q.mam
			if terms != 1 {
				if multipatterns >= eagerLimit {
					matcher = q.lazyMAM
				}
				multipatterns++
			}
			if err = matcher.AddPattern(quantifier, pattern); err != nil {
				return err
			}
		}
		q.activated[quantifier] = true
		q.activationTrail = append(q.activationTrail, quantifier)
	}
	return nil
}

func (q *SMTQuantifierManager) PushScope() error {
	if err := q.open(); err != nil {
		return err
	}
	if err := q.mam.PushScope(); err != nil {
		return err
	}
	if err := q.lazyMAM.PushScope(); err != nil {
		return errors.Join(err, q.mam.PopScopes(1))
	}
	if err := q.modelFinder.PushScope(); err != nil {
		_ = q.mam.PopScopes(1)
		_ = q.lazyMAM.PopScopes(1)
		return err
	}
	q.activationScopes = append(q.activationScopes, len(q.activationTrail))
	q.newEnodeScopes = append(q.newEnodeScopes, q.newEnodeHead)
	q.lazyMatchingScopes = append(q.lazyMatchingScopes, len(q.lazyMatchingTrail))
	return nil
}

func (q *SMTQuantifierManager) PopScopes(count uint32) error {
	if err := q.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(q.activationScopes)) {
		return ast.ErrASTDomain
	}
	level := len(q.activationScopes) - int(count)
	limit := q.activationScopes[level]
	if err := q.mam.PopScopes(count); err != nil {
		return err
	}
	if err := q.lazyMAM.PopScopes(count); err != nil {
		return err
	}
	if err := q.modelFinder.PopScopes(count); err != nil {
		return err
	}
	for _, quantifier := range q.activationTrail[limit:] {
		delete(q.activated, quantifier)
	}
	q.activationTrail = q.activationTrail[:limit]
	q.activationScopes = q.activationScopes[:level]
	q.newEnodeHead = q.newEnodeScopes[level]
	q.newEnodeScopes = q.newEnodeScopes[:level]
	for len(q.lazyMatchingTrail) > q.lazyMatchingScopes[level] {
		last := len(q.lazyMatchingTrail) - 1
		q.lazyMatchingIndex = q.lazyMatchingTrail[last]
		q.lazyMatchingTrail = q.lazyMatchingTrail[:last]
	}
	q.lazyMatchingScopes = q.lazyMatchingScopes[:level]
	return nil
}

func (q *SMTQuantifierManager) promotedEntries() (map[*SMTFingerprint]bool, error) {
	promoted := make(map[*SMTFingerprint]bool)
	if !q.promoteUnsat {
		return promoted, nil
	}
	for _, entry := range q.registry.schedule.newEntries {
		if entry.Fingerprint == nil {
			return nil, ast.ErrASTDomain
		}
		body, err := q.state.manager.GetQuantifierBody(entry.Fingerprint.data)
		if err != nil {
			return nil, err
		}
		unsat, err := q.checker.IsUnsatisfiable(body, entry.Fingerprint.args)
		if err != nil {
			return nil, err
		}
		if unsat {
			promoted[entry.Fingerprint] = true
		}
	}
	return promoted, nil
}

func (q *SMTQuantifierManager) instantiate(entries []SMTQIQueueEntry) (resultErr error) {
	for _, entry := range entries {
		prepared, err := q.builder.Prepare(entry)
		if err != nil {
			return fmt.Errorf("prepare quantified instance: %w", err)
		}
		instance, err := q.builder.MaterializeAndInternalize(prepared, q.rewriter, q.registry.statistics[entry.Fingerprint.data], q.state.scopeLevel, q.state)
		err = errors.Join(err, prepared.Close())
		if err != nil {
			return fmt.Errorf("materialize quantified instance: %w", err)
		}
		if instance == nil {
			continue
		}
		if !instance.ReducedTrue() {
			lemma, lemmaErr := instance.Lemma()
			if lemmaErr == nil {
				lemmaErr = q.registry.schedule.RecordInstance(lemma)
			}
			if lemmaErr != nil {
				return errors.Join(lemmaErr, instance.Close())
			}
		}
		if err = instance.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (q *SMTQuantifierManager) Propagate() error {
	if err := q.open(); err != nil {
		return err
	}
	if err := q.activatePatterns(); err != nil {
		return err
	}
	q.notifyNewEnodes()
	if q.ematching && q.mam.HasWork() {
		if err := q.mam.Match(); err != nil {
			return err
		}
	}
	promoted, err := q.promotedEntries()
	if err != nil {
		return err
	}
	entries, err := q.registry.schedule.ScheduleNew(promoted)
	if err != nil {
		return err
	}
	return q.instantiate(entries)
}

func (q *SMTQuantifierManager) active(quantifier ast.ASTHandle) bool {
	return q.state.isRelevant(quantifier) && q.state.context.assignments[quantifier] == lbool.True
}

func (q *SMTQuantifierManager) quickCheck() (bool, error) {
	if !q.quickUnsat && !q.quickNotSat {
		return true, nil
	}
	result := true
	for _, quantifier := range q.registry.quantifiers {
		if !q.active(quantifier) {
			continue
		}
		generated, err := q.quick.InstantiateUnsat(quantifier)
		if err != nil {
			return false, err
		}
		if generated {
			result = false
		}
	}
	if !result || !q.quickNotSat {
		if err := q.Propagate(); err != nil {
			return false, err
		}
		return result, nil
	}
	for _, quantifier := range q.registry.quantifiers {
		if !q.active(quantifier) {
			continue
		}
		generated, err := q.quick.InstantiateNotSat(quantifier)
		if err != nil {
			return false, err
		}
		if generated {
			result = false
		}
	}
	if err := q.Propagate(); err != nil {
		return false, err
	}
	return result, nil
}

func (q *SMTQuantifierManager) FinalCheck(full bool) (SMTFinalCheckStatus, error) {
	if err := q.open(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if !full {
		if q.lazyInstantiation {
			if err := q.matchLazyPatterns(); err != nil {
				return SMTFinalCheckGiveUp, err
			}
		}
		return SMTFinalCheckDone, nil
	}
	if q.registry.Empty() {
		return SMTFinalCheckDone, nil
	}
	done, entries, err := q.registry.schedule.FinalCheck()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if err = q.instantiate(entries); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if err = q.matchLazyPatterns(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if !done || q.state.canPropagate() {
		return SMTFinalCheckContinue, nil
	}
	if !q.lazyQuick {
		done, err = q.quickCheck()
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		if !done {
			return SMTFinalCheckContinue, nil
		}
	}
	return SMTFinalCheckDone, nil
}

// Source default_qm_plugin::final_check_quant rematches the separate lazy MAM
// a bounded number of times. Each increment is trailed; init_search resets the
// current counter without removing earlier scope trails.
func (q *SMTQuantifierManager) matchLazyPatterns() error {
	if !q.ematching || q.registry.Empty() || q.lazyMatchingIndex >= q.maxLazyMultipatternMatching {
		return nil
	}
	if err := q.activatePatterns(); err != nil {
		return err
	}
	if err := q.lazyMAM.Rematch(false); err != nil {
		return err
	}
	q.lazyMatchingTrail = append(q.lazyMatchingTrail, q.lazyMatchingIndex)
	q.lazyMatchingIndex++
	return nil
}

func (q *SMTQuantifierManager) IsShared(node *SMTEnode) bool {
	return q != nil && !q.closed && len(q.activated) != 0 && (q.mam.IsShared(node) || q.lazyMAM.IsShared(node))
}

func (q *SMTQuantifierManager) NotifyRelevant(node *SMTEnode) {
	if q != nil && !q.closed && q.ematching && !q.registry.Empty() {
		q.mam.Relevant(node)
		q.lazyMAM.Relevant(node)
	}
}

// Source quantifier_manager::check_model is called after ground final checking,
// separately from the round-robin full quantifier-instantiation slot.
type SMTQuantifierModelCheckResult uint8

const (
	SMTQuantifierModelSAT SMTQuantifierModelCheckResult = iota
	SMTQuantifierModelUnknown
	SMTQuantifierModelRestart
)

// CheckModel validates and retains a candidate only after the context's ground
// final-check cycle has completed. It reports UNKNOWN when MBQI is unavailable.
func (q *SMTQuantifierManager) CheckModel() (SMTQuantifierModelCheckResult, error) {
	if err := q.open(); err != nil {
		return SMTQuantifierModelUnknown, err
	}
	if q.registry.Empty() {
		return SMTQuantifierModelSAT, nil
	}
	if q.mbqi && q.modelChecker != nil && q.candidateModel != nil {
		generator, err := q.candidateModel()
		if err != nil {
			return SMTQuantifierModelUnknown, err
		}
		if len(q.modelFinder.pendingConstraints) != 0 {
			return SMTQuantifierModelRestart, generator.Close()
		}
		if err = q.modelChecker.SetCandidateTerms(generator.rootValues); err != nil {
			return SMTQuantifierModelUnknown, errors.Join(err, generator.Close())
		}
		active, activeErr := q.activeQuantifiers()
		if activeErr != nil {
			return SMTQuantifierModelUnknown, errors.Join(activeErr, generator.Close())
		}
		modelSatisfied, checkErr := q.modelChecker.Check(generator.model, active)
		if checkErr != nil {
			return SMTQuantifierModelUnknown, errors.Join(checkErr, generator.Close())
		}
		if !modelSatisfied {
			if err = generator.Close(); err != nil {
				return SMTQuantifierModelUnknown, err
			}
			if q.modelChecker.HasNewInstances() {
				return SMTQuantifierModelRestart, nil
			}
			return SMTQuantifierModelUnknown, nil
		}
		if q.acceptedModel != nil {
			if err = q.acceptedModel.Close(); err != nil {
				return SMTQuantifierModelUnknown, errors.Join(err, generator.Close())
			}
		}
		q.acceptedModel = generator
		return SMTQuantifierModelSAT, nil
	}
	return SMTQuantifierModelUnknown, nil
}

func (q *SMTQuantifierManager) mbqiEnabled(quantifier ast.ASTHandle) (bool, error) {
	if q.mbqiID == "" {
		return true, nil
	}
	identifier, _, err := q.state.manager.GetQuantifierIDs(quantifier)
	if err != nil {
		return false, err
	}
	if identifier == (usymbol.Symbol{}) || identifier.IsNumerical() {
		return false, nil
	}
	text, present, err := q.state.manager.Symbols.BareString(identifier)
	return present && strings.HasPrefix(text, q.mbqiID), err
}

func (q *SMTQuantifierManager) activeQuantifiers() ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, 0, len(q.registry.quantifiers))
	for _, quantifier := range q.registry.quantifiers {
		enabled, err := q.mbqiEnabled(quantifier)
		if err != nil {
			return nil, err
		}
		if q.active(quantifier) && enabled {
			result = append(result, quantifier)
		}
	}
	return result, nil
}

func (q *SMTQuantifierManager) Close() error {
	if q == nil || q.state == nil {
		return ast.ErrASTHandle
	}
	if q.closed {
		return nil
	}
	if q.acceptedModel != nil {
		if err := q.acceptedModel.Close(); err != nil {
			return err
		}
		q.acceptedModel = nil
	}
	err := q.substitute.Close()
	err = errors.Join(err, q.modelFinder.Close())
	if q.modelChecker != nil {
		err = errors.Join(err, q.modelChecker.Close())
	}
	if err == nil {
		if q.state.quantifierManager == q {
			q.state.quantifierManager = nil
		}
		q.closed = true
		q.builder = nil
		q.mam = nil
		q.lazyMAM = nil
		q.modelFinder = nil
		q.modelChecker = nil
		q.candidateModel = nil
		q.quick = nil
		q.checker = nil
		q.rewriter = nil
		q.registry = nil
		q.state = nil
	}
	return err
}
