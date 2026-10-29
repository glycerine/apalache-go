package apalache

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type SafetyOutcome uint8

const (
	SafetyUnknown SafetyOutcome = iota
	SafetyNoCounterexample
	SafetyCounterexample
	SafetyDeadlock
	SafetyExecutionsTooShort
)

type BoundedSafetyOptions struct {
	MaxSteps              int
	CheckDeadlocks        bool
	DiscardDisabled       *bool
	SMTEncoding           SMTEncoding
	InvariantMinimumDepth int
	Tuning                CheckerTuning
	MaxErrors             int
	Listeners             []ModelCheckerListener
	RuleStats             *RuleStatListener
	RewriteStats          SymbolicStateRewriterListener
	excludedViews         [][]Expr // exclusions retained by the enclosing simulation search
}

type BoundedCounterexample struct {
	Module          *Module
	Depth           int
	Invariant       string
	Deadlock        bool
	ActionInvariant bool
	TraceInvariant  bool
	InitTransition  int
	NextTransitions []int
	States          []DecodedState
	View            []Expr
}

// BoundedSafetyResult describes either a completed bounded search or the
// first counterexample found in normalized transition order.
type BoundedSafetyResult struct {
	Outcome            SafetyOutcome
	Depth              int
	Invariant          string
	ActionInvariant    bool
	TraceInvariant     bool
	InitTransition     int
	NextTransitions    []int
	ExploredExecutions int
	States             []DecodedState
	Counterexamples    []BoundedCounterexample
	ExampleStates      []DecodedState
	initialStateFound  bool
}

// CheckBoundedSafety checks every normalized transition path through maxSteps
// applications of Next. The initial state is depth 0.
func CheckBoundedSafety(
	ctx context.Context,
	module *Module,
	predicates DerivedPredicates,
	maxSteps int,
) (BoundedSafetyResult, error) {
	return CheckBoundedSafetyWithOptions(ctx, module, predicates, BoundedSafetyOptions{
		MaxSteps: maxSteps, CheckDeadlocks: true,
	})
}

func CheckBoundedSafetyWithOptions(
	ctx context.Context,
	module *Module,
	predicates DerivedPredicates,
	options BoundedSafetyOptions,
) (BoundedSafetyResult, error) {
	if module == nil {
		return BoundedSafetyResult{}, fmt.Errorf("cannot check a nil module")
	}
	if !module.HasProperty(PropertyTransitionsFound) {
		return BoundedSafetyResult{}, fmt.Errorf("bounded safety checking requires normalized transitions")
	}
	if options.MaxSteps < 0 {
		return BoundedSafetyResult{}, fmt.Errorf("maximum step count cannot be negative")
	}
	if options.InvariantMinimumDepth < 0 {
		return BoundedSafetyResult{}, fmt.Errorf("invariant minimum depth cannot be negative")
	}
	if options.MaxErrors < 0 {
		return BoundedSafetyResult{}, fmt.Errorf("maximum error count cannot be negative")
	}
	if options.MaxErrors == 0 {
		options.MaxErrors = 1
	}
	if options.MaxErrors > 1 && predicates.View == "" {
		return BoundedSafetyResult{}, fmt.Errorf("maximum error count greater than one requires a state view")
	}
	smtEncoding := options.SMTEncoding
	if smtEncoding == "" {
		smtEncoding = SMTEncodingOOPSLA19
	}
	if smtEncoding != SMTEncodingOOPSLA19 && smtEncoding != SMTEncodingArrays && smtEncoding != SMTEncodingFunArrays {
		return BoundedSafetyResult{}, fmt.Errorf("unsupported checker SMT encoding %s", smtEncoding)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	initTransitions := TransitionsFromModule(module, NormalizedInitPrefix)
	if len(initTransitions) == 0 {
		return BoundedSafetyResult{}, fmt.Errorf("module has no normalized initial transitions")
	}
	nextTransitions := TransitionsFromModule(module, NormalizedNextPrefix)
	var stateInvariants []namedExpression
	var actionInvariants []namedExpression
	var traceInvariants []namedTraceInvariant
	normalizedVCs := false
	var err error
	if len(predicates.Invariants) > 0 {
		stateInvariants, actionInvariants, traceInvariants, normalizedVCs, err = normalizedCheckerVCs(module)
		if err != nil {
			return BoundedSafetyResult{}, err
		}
		if normalizedVCs && len(predicates.Invariants) == 1 {
			for index := range stateInvariants {
				stateInvariants[index].name = predicates.Invariants[0]
			}
			for index := range actionInvariants {
				actionInvariants[index].name = predicates.Invariants[0]
			}
			for index := range traceInvariants {
				traceInvariants[index].name = predicates.Invariants[0]
			}
		} else if normalizedVCs {
			if len(predicates.vcStateSources) == len(stateInvariants) {
				for index := range stateInvariants {
					stateInvariants[index].name = predicates.vcStateSources[index]
				}
			}
			if len(predicates.vcActionSources) == len(actionInvariants) {
				for index := range actionInvariants {
					actionInvariants[index].name = predicates.vcActionSources[index]
				}
			}
			if len(predicates.vcTraceSources) == len(traceInvariants) {
				for index := range traceInvariants {
					traceInvariants[index].name = predicates.vcTraceSources[index]
				}
			}
		}
	}
	if !normalizedVCs {
		stateInvariants = make([]namedExpression, 0, len(predicates.Invariants))
		actionInvariants = make([]namedExpression, 0, len(predicates.Invariants))
		traceInvariants = make([]namedTraceInvariant, 0, len(predicates.Invariants))
	}
	levelFinder := NewLevelFinder(module)
	for _, name := range predicates.Invariants {
		if normalizedVCs {
			break
		}
		declaration, found := declarationNamed(module, name)
		operator, ok := declaration.(*OperDecl)
		if !found || !ok {
			return BoundedSafetyResult{}, fmt.Errorf("invariant %s was not found", name)
		}
		parameters := operator.FormalParams()
		if len(parameters) == 1 && parameters[0].Arity <= 0 {
			if levelFinder.LevelOfDeclaration(operator) != ConstantLevel {
				return BoundedSafetyResult{}, fmt.Errorf("trace invariant %s refers to state variables or action/temporal operators", name)
			}
			historyType, err := validateTraceInvariantType(operator, module.VarDeclarations())
			if err != nil {
				return BoundedSafetyResult{}, err
			}
			violation, err := invariantViolationExpression(operator.Body())
			if err != nil {
				return BoundedSafetyResult{}, fmt.Errorf("prepare trace invariant %s violation: %w", name, err)
			}
			traceInvariants = append(traceInvariants, namedTraceInvariant{
				name: name, parameter: parameters[0].Name, expression: violation,
				historyType: historyType,
			})
			continue
		}
		if len(parameters) != 0 {
			return BoundedSafetyResult{}, fmt.Errorf("invariant %s has %d parameters; expected zero or one trace parameter", name, len(parameters))
		}
		body := invariantAssignmentsToEquality(operator.Body())
		violation, err := invariantViolationExpression(body)
		if err != nil {
			return BoundedSafetyResult{}, fmt.Errorf("prepare invariant %s violation: %w", name, err)
		}
		named := namedExpression{name: name, expression: violation}
		switch levelFinder.LevelOfExpression(body) {
		case ConstantLevel, StateLevel:
			stateInvariants = append(stateInvariants, named)
		case ActionLevel:
			actionInvariants = append(actionInvariants, named)
		case TemporalLevel:
			return BoundedSafetyResult{}, fmt.Errorf("invariant %s is temporal, expected a state or action predicate", name)
		}
	}
	stateConstraints, err := namedOperatorExpressions(module, predicates.StateConstraints, "state constraint")
	if err != nil {
		return BoundedSafetyResult{}, err
	}
	actionConstraints, err := namedOperatorExpressions(module, predicates.ActionConstraints, "action constraint")
	if err != nil {
		return BoundedSafetyResult{}, err
	}
	nextEnabled := projectedTransitionEnabledness(nextTransitions, actionConstraints, stateConstraints, module.VarDeclarations())
	constants := declarationNameSlice(module.ConstDeclarations())
	variables := declarationNameSlice(module.VarDeclarations())
	variableDeclarations := module.VarDeclarations()
	constantInit, hasConstantInit := OperatorBody(module, NormalizedConstInit)
	assumptions := module.AssumeDeclarations()
	view, err := checkerStateView(module, predicates)
	if err != nil {
		return BoundedSafetyResult{}, err
	}

	result := BoundedSafetyResult{Outcome: SafetyNoCounterexample, Depth: -1, InitTransition: -1}
	inputs := modulePathInputs{
		module:    module,
		constants: constants, variables: variables, constantInit: constantInit,
		hasConstantInit: hasConstantInit, assumptions: assumptions,
		initTransitions: initTransitions, nextTransitions: nextTransitions, nextEnabled: nextEnabled,
		stateInvariants: stateInvariants, actionInvariants: actionInvariants,
		traceInvariants: traceInvariants, variableDeclarations: variableDeclarations,
		maxSteps: options.MaxSteps, checkDeadlocks: options.CheckDeadlocks,
		discardDisabled:       options.DiscardDisabled == nil || *options.DiscardDisabled,
		smtEncoding:           smtEncoding,
		invariantMinimumDepth: options.InvariantMinimumDepth,
		stateConstraints:      stateConstraints, actionConstraints: actionConstraints,
		tuning: options.Tuning,
		view:   view, maxErrors: options.MaxErrors,
		excludedViews: options.excludedViews,
		listeners:     append([]ModelCheckerListener(nil), options.Listeners...),
		ruleStats:     options.RuleStats,
		rewriteStats:  options.RewriteStats,
	}
	queue := make([]boundedPath, 0, len(initTransitions))
	for initNumber := range initTransitions {
		if options.Tuning.allowsTransition(0, initNumber) {
			queue = append(queue, boundedPath{init: initNumber})
		}
	}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		children := make([]boundedPath, 0, len(nextTransitions))
		found, err := searchSafetyPath(ctx, inputs, path, &result, &children)
		if err != nil {
			return BoundedSafetyResult{}, err
		}
		if found {
			return result, nil
		}
		queue = append(queue, children...)
	}
	if !result.initialStateFound {
		// No user state exists to decode or send to trace listeners.
		result.Outcome = SafetyExecutionsTooShort
		if options.CheckDeadlocks {
			result.Outcome = SafetyDeadlock
		}
	}
	return result, nil
}

func checkerStateView(module *Module, predicates DerivedPredicates) (Expr, error) {
	if view, found := OperatorBody(module, NormalizedVCView); found {
		return view, nil
	}
	if predicates.View != "" {
		if view, found := OperatorBody(module, predicates.View); found {
			return view, nil
		}
		return nil, fmt.Errorf("state view %s was not found", predicates.View)
	}
	return nil, nil
}

func normalizedCheckerVCs(module *Module) (
	[]namedExpression,
	[]namedExpression,
	[]namedTraceInvariant,
	bool,
	error,
) {
	type vcPair struct{ positive, negative *OperDecl }
	pairs := func(positivePrefix, negativePrefix string) ([]vcPair, error) {
		positives, negatives := make([]*OperDecl, 0), make([]*OperDecl, 0)
		for _, declaration := range module.OperDeclarations() {
			if strings.HasPrefix(declaration.Name(), positivePrefix) {
				positives = append(positives, declaration)
			} else if strings.HasPrefix(declaration.Name(), negativePrefix) {
				negatives = append(negatives, declaration)
			}
		}
		if len(positives) != len(negatives) {
			return nil, fmt.Errorf("verification condition counts for %s and %s differ: %d and %d", positivePrefix, negativePrefix, len(positives), len(negatives))
		}
		result := make([]vcPair, len(positives))
		for index := range positives {
			result[index] = vcPair{positive: positives[index], negative: negatives[index]}
		}
		return result, nil
	}
	statePairs, err := pairs(NormalizedVCInvariantPrefix, NormalizedVCNotInvariantPrefix)
	if err != nil {
		return nil, nil, nil, true, err
	}
	actionPairs, err := pairs(NormalizedVCActionPrefix, NormalizedVCNotActionPrefix)
	if err != nil {
		return nil, nil, nil, true, err
	}
	tracePairs, err := pairs(NormalizedVCTracePrefix, NormalizedVCNotTracePrefix)
	if err != nil {
		return nil, nil, nil, true, err
	}
	state := make([]namedExpression, len(statePairs))
	for index, pair := range statePairs {
		state[index] = namedExpression{name: pair.positive.Name(), expression: pair.negative.Body()}
	}
	action := make([]namedExpression, len(actionPairs))
	for index, pair := range actionPairs {
		action[index] = namedExpression{name: pair.positive.Name(), expression: pair.negative.Body()}
	}
	trace := make([]namedTraceInvariant, 0, len(tracePairs))
	for _, pair := range tracePairs {
		parameters := pair.positive.FormalParams()
		if len(parameters) != 1 || parameters[0].Arity > 0 {
			return nil, nil, nil, true, fmt.Errorf("trace verification condition %s has invalid parameters", pair.positive.Name())
		}
		historyType, validationErr := validateTraceInvariantType(pair.positive, module.VarDeclarations())
		if validationErr != nil {
			return nil, nil, nil, true, validationErr
		}
		trace = append(trace, namedTraceInvariant{
			name: pair.positive.Name(), parameter: parameters[0].Name,
			expression: pair.negative.Body(), historyType: historyType,
		})
	}
	found := len(statePairs)+len(actionPairs)+len(tracePairs) > 0
	return state, action, trace, found, nil
}

func invariantAssignmentsToEquality(expression Expr) Expr {
	return RewriteExpressions(nil, func(current Expr) (Expr, bool) {
		assignment, ok := current.(*OperExpr)
		if !ok || assignment.Operator().Name() != "Apalache!:=" {
			return nil, false
		}
		return MustOperExpr(Op("EQ"), assignment.Tag(), assignment.Arguments()...), true
	})(expression)
}

func invariantViolationExpression(expression Expr) (Expr, error) {
	original := RemoveSkolemizationMarkers(expression, nil)
	negated := MustOperExpr(Op("NOT"), Typed(BoolT1), original)
	normalized, err := NormalizeNNF(negated, nil)
	if err != nil {
		return nil, err
	}
	return MarkSkolemizableExpression(normalized, nil), nil
}

type namedExpression struct {
	name       string
	expression Expr
}

type namedTraceInvariant struct {
	name        string
	parameter   string
	expression  Expr
	historyType SeqType
}

func validateTraceInvariantType(declaration *OperDecl, variables []*VarDecl) (SeqType, error) {
	typ, typed := declaration.Tag().Type()
	operatorType, ok := typ.(OperatorType)
	if !typed || !ok || len(operatorType.Arguments()) != 1 || !EqualTypes(operatorType.Result(), BoolT1) {
		return SeqType{}, fmt.Errorf("trace invariant %s must be a Boolean predicate over a history sequence", declaration.Name())
	}
	historyType, ok := operatorType.Arguments()[0].(SeqType)
	if !ok {
		return SeqType{}, fmt.Errorf("trace invariant %s parameter must be a sequence of state records", declaration.Name())
	}
	var fields map[string]Type
	switch recordType := historyType.Element().(type) {
	case RecordType:
		fields = recordType.Fields()
	case RecordRowType:
		if _, open := recordType.Row().Other(); open {
			return SeqType{}, fmt.Errorf("trace invariant %s history record must be closed", declaration.Name())
		}
		fields = recordType.Row().Fields()
	default:
		return SeqType{}, fmt.Errorf("trace invariant %s parameter must be a sequence of state records", declaration.Name())
	}
	expected := make(map[string]Type, len(variables))
	for _, variable := range variables {
		variableType, ok := variable.Tag().Type()
		if !ok {
			return SeqType{}, fmt.Errorf("state variable %s is untyped", variable.Name())
		}
		expected[variable.Name()] = variableType
	}
	if len(fields) != len(expected) {
		return SeqType{}, fmt.Errorf("trace invariant %s history record has fields %v, expected state variables %v",
			declaration.Name(), sortedStringKeys(fields), sortedStringKeys(expected))
	}
	for name, expectedType := range expected {
		actualType, found := fields[name]
		if !found || !EqualTypes(actualType, expectedType) {
			return SeqType{}, fmt.Errorf("trace invariant %s history field %s has type %v, expected %s",
				declaration.Name(), name, actualType, expectedType)
		}
	}
	return historyType, nil
}

type boundedPath struct {
	init int
	next []int
}

type modulePathInputs struct {
	module                *Module
	constants             []string
	variables             []string
	constantInit          Expr
	hasConstantInit       bool
	assumptions           []*AssumeDecl
	initTransitions       []Expr
	nextTransitions       []Expr
	nextEnabled           []Expr
	stateInvariants       []namedExpression
	actionInvariants      []namedExpression
	traceInvariants       []namedTraceInvariant
	variableDeclarations  []*VarDecl
	stateConstraints      []namedExpression
	actionConstraints     []namedExpression
	maxSteps              int
	checkDeadlocks        bool
	discardDisabled       bool
	smtEncoding           SMTEncoding
	invariantMinimumDepth int
	tuning                CheckerTuning
	view                  Expr
	excludedViews         [][]Expr
	maxErrors             int
	listeners             []ModelCheckerListener
	ruleStats             *RuleStatListener
	rewriteStats          SymbolicStateRewriterListener
}

func searchSafetyPath(
	ctx context.Context,
	inputs modulePathInputs,
	path boundedPath,
	result *BoundedSafetyResult,
	children *[]boundedPath,
) (bool, error) {
	executor, err := replaySafetyPath(inputs, path)
	if err != nil {
		if errors.Is(err, errDisabledTransition) {
			return false, nil
		}
		return false, err
	}
	result.ExploredExecutions++
	status, err := executor.Check(ctx)
	if err != nil {
		return false, err
	}
	if status == SMTUnknown {
		return stopOnUnknown(result), nil
	}
	if status == SMTUnsatisfiable {
		return false, nil
	}
	if len(path.next) == 0 {
		result.initialStateFound = true
	}
	if len(path.next) >= inputs.invariantMinimumDepth {
		for invariantNumber, invariant := range inputs.stateInvariants {
			if !inputs.tuning.allowsInvariant(len(path.next), "state", invariantNumber) {
				continue
			}
			for {
				status, states, views, checkErr := executor.checkAndDecodeDistinctState(
					ctx, invariant.expression, inputs.view, safetyExcludedViews(result))
				if checkErr != nil {
					return false, fmt.Errorf("check invariant %s at depth %d: %w", invariant.name, len(path.next), checkErr)
				}
				if status == SMTUnknown {
					return stopOnUnknown(result), nil
				}
				if status != SMTSatisfiable {
					break
				}
				if err = recordSafetyCounterexample(result, BoundedCounterexample{
					Module: inputs.module,
					Depth:  len(path.next), Invariant: invariant.name, InitTransition: path.init,
					NextTransitions: append([]int(nil), path.next...), States: states, View: views,
				}, inputs.listeners); err != nil {
					return false, err
				}
				if len(result.Counterexamples) >= inputs.maxErrors {
					return true, nil
				}
			}
		}
		// Scala checks trace invariants independently of the state/action filter.
		for _, invariant := range inputs.traceInvariants {
			for {
				status, states, views, checkErr := executor.checkTraceAndDecodeDistinctView(
					ctx, invariant.parameter, invariant.expression, invariant.historyType,
					inputs.variableDeclarations, inputs.view, safetyExcludedViews(result))
				if checkErr != nil {
					return false, fmt.Errorf("check trace invariant %s at depth %d: %w", invariant.name, len(path.next), checkErr)
				}
				if status == SMTUnknown {
					return stopOnUnknown(result), nil
				}
				if status != SMTSatisfiable {
					break
				}
				if err = recordSafetyCounterexample(result, BoundedCounterexample{
					Module: inputs.module,
					Depth:  len(path.next), Invariant: invariant.name, TraceInvariant: true,
					InitTransition: path.init, NextTransitions: append([]int(nil), path.next...),
					States: states, View: views,
				}, inputs.listeners); err != nil {
					return false, err
				}
				if len(result.Counterexamples) >= inputs.maxErrors {
					return true, nil
				}
			}
		}
	}
	if inputs.tuning.OutputTraces() && len(path.next) == inputs.maxSteps && result.ExampleStates == nil {
		truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
		status, states, decodeErr := executor.CheckAndDecode(ctx, truth)
		if decodeErr != nil {
			return false, decodeErr
		}
		if status == SMTUnknown {
			return stopOnUnknown(result), nil
		}
		if status == SMTSatisfiable {
			result.ExampleStates = states
			if err = notifyExampleListeners(inputs.listeners, states, 1); err != nil {
				return false, err
			}
		}
	}
	if len(path.next) == inputs.maxSteps && !inputs.checkDeadlocks {
		return false, nil
	}
	enabledNext := false
	for nextNumber := range inputs.nextTransitions {
		if !inputs.tuning.allowsTransition(len(path.next)+1, nextNumber) {
			continue
		}
		child := boundedPath{init: path.init, next: append(append([]int(nil), path.next...), nextNumber)}
		childExecutor, childErr := replayPendingNext(inputs, path, nextNumber)
		if childErr != nil {
			if errors.Is(childErr, errDisabledTransition) {
				continue
			}
			return false, childErr
		}
		if !inputs.discardDisabled && !inputs.checkDeadlocks && len(inputs.actionInvariants) == 0 {
			if len(path.next) < inputs.maxSteps {
				*children = append(*children, child)
			}
			continue
		}
		childStatus, childErr := childExecutor.Check(ctx)
		if childErr != nil {
			return false, childErr
		}
		if childStatus == SMTUnknown {
			return stopOnUnknown(result), nil
		}
		if childStatus == SMTUnsatisfiable {
			continue
		}
		enabledNext = true
		if len(path.next) == inputs.maxSteps {
			// The outgoing transition is only an enabledness probe at the
			// boundary. Its action is outside the requested checking bound.
			continue
		}
		for invariantNumber, invariant := range inputs.actionInvariants {
			if !inputs.tuning.allowsInvariant(len(child.next), "action", invariantNumber) {
				continue
			}
			for {
				status, states, views, invariantErr := childExecutor.checkPendingAndDecodeDistinctView(
					ctx, invariant.expression, inputs.view, safetyExcludedViews(result))
				if invariantErr != nil {
					return false, fmt.Errorf("check action invariant %s at depth %d: %w", invariant.name, len(child.next), invariantErr)
				}
				if status == SMTUnknown {
					return stopOnUnknown(result), nil
				}
				if status != SMTSatisfiable {
					break
				}
				if err = recordSafetyCounterexample(result, BoundedCounterexample{
					Module: inputs.module,
					Depth:  len(child.next), Invariant: invariant.name, ActionInvariant: true,
					InitTransition: child.init, NextTransitions: append([]int(nil), child.next...),
					States: states, View: views,
				}, inputs.listeners); err != nil {
					return false, err
				}
				if len(result.Counterexamples) >= inputs.maxErrors {
					return true, nil
				}
			}
		}
		if len(path.next) < inputs.maxSteps {
			*children = append(*children, child)
		}
	}
	if inputs.checkDeadlocks && inputs.nextEnabled != nil {
		enabled := make([]Expr, 0, len(inputs.nextEnabled))
		for number, expression := range inputs.nextEnabled {
			if inputs.tuning.allowsTransition(len(path.next)+1, number) {
				enabled = append(enabled, expression)
			}
		}
		deadlockExpression := MustOperExpr(Op("NOT"), Typed(BoolT1), booleanOr(enabled...))
		status, states, decodeErr := executor.CheckAndDecode(ctx, deadlockExpression)
		if decodeErr != nil {
			return false, fmt.Errorf("check projected deadlock at depth %d: %w", len(path.next), decodeErr)
		}
		if status == SMTUnknown {
			result.Outcome = SafetyUnknown
			return true, nil
		}
		if status == SMTSatisfiable {
			if len(result.Counterexamples) != 0 {
				return false, nil
			}
			return recordDeadlock(inputs, path, result, states)
		}
		return false, nil
	}
	if inputs.checkDeadlocks && !enabledNext {
		if len(result.Counterexamples) != 0 {
			return false, nil
		}
		truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
		status, states, decodeErr := executor.CheckAndDecode(ctx, truth)
		if decodeErr != nil {
			return false, decodeErr
		}
		if status == SMTUnknown {
			result.Outcome = SafetyUnknown
			return true, nil
		}
		return recordDeadlock(inputs, path, result, states)
	}
	return false, nil
}

func recordDeadlock(inputs modulePathInputs, path boundedPath, result *BoundedSafetyResult, states []DecodedState) (bool, error) {
	result.Outcome = SafetyDeadlock
	result.Depth = len(path.next)
	result.InitTransition = path.init
	result.NextTransitions = append([]int(nil), path.next...)
	result.States = states
	deadlock := BoundedCounterexample{
		Module: inputs.module,
		Depth:  len(path.next), Deadlock: true, InitTransition: path.init,
		NextTransitions: append([]int(nil), path.next...), States: states,
	}
	if err := notifyCounterexampleListeners(inputs.listeners, deadlock, 0); err != nil {
		return false, err
	}
	return true, nil
}

func safetyExcludedViews(result *BoundedSafetyResult) [][]Expr {
	views := make([][]Expr, 0, len(result.Counterexamples))
	for _, counterexample := range result.Counterexamples {
		if len(counterexample.View) != 0 {
			views = append(views, counterexample.View)
		}
	}
	return views
}

func recordSafetyCounterexample(
	result *BoundedSafetyResult,
	counterexample BoundedCounterexample,
	listeners []ModelCheckerListener,
) error {
	result.Counterexamples = append(result.Counterexamples, counterexample)
	result.Outcome = SafetyCounterexample
	if len(result.Counterexamples) == 1 {
		result.Depth = counterexample.Depth
		result.Invariant = counterexample.Invariant
		result.ActionInvariant = counterexample.ActionInvariant
		result.TraceInvariant = counterexample.TraceInvariant
		result.InitTransition = counterexample.InitTransition
		result.NextTransitions = append([]int(nil), counterexample.NextTransitions...)
		result.States = counterexample.States
	}
	if err := notifyCounterexampleListeners(listeners, counterexample, len(result.Counterexamples)); err != nil {
		return err
	}
	return nil
}

func notifyCounterexampleListeners(
	listeners []ModelCheckerListener,
	counterexample BoundedCounterexample,
	index int,
) error {
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		if err := listener.OnCounterexample(cloneBoundedCounterexample(counterexample), index); err != nil {
			return fmt.Errorf("notify counterexample listener: %w", err)
		}
	}
	return nil
}

func notifyExampleListeners(listeners []ModelCheckerListener, states []DecodedState, index int) error {
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		if err := listener.OnExample(cloneDecodedStates(states), index); err != nil {
			return fmt.Errorf("notify example listener: %w", err)
		}
	}
	return nil
}

func stopOnUnknown(result *BoundedSafetyResult) bool {
	if len(result.Counterexamples) == 0 {
		result.Outcome = SafetyUnknown
	}
	return true
}

func replayPendingNext(inputs modulePathInputs, path boundedPath, nextNumber int) (*TransitionExecutor, error) {
	executor, err := replaySafetyPath(inputs, path)
	if err != nil {
		return nil, err
	}
	if err = selectPathTransition(executor, nextNumber, inputs.nextTransitions,
		fmt.Sprintf("next at depth %d", len(path.next)+1)); err != nil {
		return nil, err
	}
	if err = assertNamedExpressions(executor, inputs.actionConstraints); err != nil {
		return nil, err
	}
	for _, constraint := range inputs.stateConstraints {
		if err = executor.AssertNextState(constraint.expression); err != nil {
			return nil, fmt.Errorf("assert target-state constraint %s: %w", constraint.name, err)
		}
	}
	bindings := append(append([]Binding(nil), executor.states...), executor.scope.Binding.Shift(executor.constants))
	if err = executor.assertExcludedViews(inputs.view, inputs.excludedViews, bindings); err != nil {
		return nil, err
	}
	return executor, nil
}

func replaySafetyPath(inputs modulePathInputs, path boundedPath) (*TransitionExecutor, error) {
	executor := newTransitionExecutorWithProfilers(
		inputs.constants, inputs.variables, inputs.tuning.solverUIntOptions, inputs.tuning.solverTextOptions,
		inputs.tuning.shortCircuit, inputs.smtEncoding, inputs.ruleStats, inputs.rewriteStats)
	if inputs.hasConstantInit {
		if err := executor.InitializeConstants(inputs.constantInit); err != nil {
			return nil, err
		}
	}
	for _, assumption := range inputs.assumptions {
		if err := executor.AssertState(assumption.Body()); err != nil {
			return nil, fmt.Errorf("assert assumption %s: %w", assumption.Name(), err)
		}
	}
	if err := selectPathTransition(executor, path.init, inputs.initTransitions, "initial"); err != nil {
		return nil, err
	}
	if err := executor.NextState(); err != nil {
		return nil, err
	}
	if err := assertNamedExpressions(executor, inputs.stateConstraints); err != nil {
		return nil, err
	}
	for depth, number := range path.next {
		if err := selectPathTransition(executor, number, inputs.nextTransitions, fmt.Sprintf("next at depth %d", depth+1)); err != nil {
			return nil, err
		}
		if err := assertNamedExpressions(executor, inputs.actionConstraints); err != nil {
			return nil, err
		}
		if err := executor.NextState(); err != nil {
			return nil, err
		}
		if err := assertNamedExpressions(executor, inputs.stateConstraints); err != nil {
			return nil, err
		}
	}
	if err := executor.assertExcludedViews(inputs.view, inputs.excludedViews, executor.states); err != nil {
		return nil, err
	}
	return executor, nil
}

func selectPathTransition(executor *TransitionExecutor, number int, transitions []Expr, kind string) error {
	if number < 0 || number >= len(transitions) {
		return fmt.Errorf("%s transition number %d is out of range", kind, number)
	}
	enabled, err := executor.PrepareTransition(number, transitions[number])
	if err != nil {
		return err
	}
	if !enabled {
		return fmt.Errorf("%w: %s transition %d has incomplete assignments", errDisabledTransition, kind, number)
	}
	return executor.AssumeTransition(number)
}

var errDisabledTransition = errors.New("disabled transition")

func namedOperatorExpressions(module *Module, names []string, role string) ([]namedExpression, error) {
	result := make([]namedExpression, len(names))
	for index, name := range names {
		body, found := OperatorBody(module, name)
		if !found {
			return nil, fmt.Errorf("%s %s was not found", role, name)
		}
		result[index] = namedExpression{name: name, expression: body}
	}
	return result, nil
}

func assertNamedExpressions(executor *TransitionExecutor, expressions []namedExpression) error {
	for _, expression := range expressions {
		if err := executor.AssertState(expression.expression); err != nil {
			return fmt.Errorf("assert %s: %w", expression.name, err)
		}
	}
	return nil
}

func declarationNameSlice[T interface{ Name() string }](declarations []T) []string {
	result := make([]string, len(declarations))
	for index, declaration := range declarations {
		result[index] = declaration.Name()
	}
	return result
}
