package apalache

import (
	"context"
	"errors"
	"fmt"
)

type SafetyOutcome uint8

const (
	SafetyUnknown SafetyOutcome = iota
	SafetyNoCounterexample
	SafetyCounterexample
	SafetyDeadlock
)

type BoundedSafetyOptions struct {
	MaxSteps              int
	CheckDeadlocks        bool
	InvariantMinimumDepth int
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
	if ctx == nil {
		ctx = context.Background()
	}
	initTransitions := TransitionsFromModule(module, NormalizedInitPrefix)
	if len(initTransitions) == 0 {
		return BoundedSafetyResult{}, fmt.Errorf("module has no normalized initial transitions")
	}
	nextTransitions := TransitionsFromModule(module, NormalizedNextPrefix)
	stateInvariants := make([]namedExpression, 0, len(predicates.Invariants))
	actionInvariants := make([]namedExpression, 0, len(predicates.Invariants))
	traceInvariants := make([]namedTraceInvariant, 0, len(predicates.Invariants))
	levelFinder := NewLevelFinder(module)
	for _, name := range predicates.Invariants {
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
	constants := declarationNameSlice(module.ConstDeclarations())
	variables := declarationNameSlice(module.VarDeclarations())
	variableDeclarations := module.VarDeclarations()
	constantInit, hasConstantInit := OperatorBody(module, NormalizedConstInit)
	assumptions := module.AssumeDeclarations()

	result := BoundedSafetyResult{Outcome: SafetyNoCounterexample, Depth: -1, InitTransition: -1}
	inputs := modulePathInputs{
		constants: constants, variables: variables, constantInit: constantInit,
		hasConstantInit: hasConstantInit, assumptions: assumptions,
		initTransitions: initTransitions, nextTransitions: nextTransitions,
		stateInvariants: stateInvariants, actionInvariants: actionInvariants,
		traceInvariants: traceInvariants, variableDeclarations: variableDeclarations,
		maxSteps: options.MaxSteps, checkDeadlocks: options.CheckDeadlocks,
		invariantMinimumDepth: options.InvariantMinimumDepth,
		stateConstraints:      stateConstraints, actionConstraints: actionConstraints,
	}
	queue := make([]boundedPath, len(initTransitions))
	for initNumber := range initTransitions {
		queue[initNumber] = boundedPath{init: initNumber}
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
	return result, nil
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
	constants             []string
	variables             []string
	constantInit          Expr
	hasConstantInit       bool
	assumptions           []*AssumeDecl
	initTransitions       []Expr
	nextTransitions       []Expr
	stateInvariants       []namedExpression
	actionInvariants      []namedExpression
	traceInvariants       []namedTraceInvariant
	variableDeclarations  []*VarDecl
	stateConstraints      []namedExpression
	actionConstraints     []namedExpression
	maxSteps              int
	checkDeadlocks        bool
	invariantMinimumDepth int
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
		result.Outcome = SafetyUnknown
		return true, nil
	}
	if status == SMTUnsatisfiable {
		return false, nil
	}
	if len(path.next) >= inputs.invariantMinimumDepth {
		for _, invariant := range inputs.stateInvariants {
			var states []DecodedState
			status, states, err = executor.CheckAndDecode(ctx, invariant.expression)
			if err != nil {
				return false, fmt.Errorf("check invariant %s at depth %d: %w", invariant.name, len(path.next), err)
			}
			if status == SMTUnknown {
				result.Outcome = SafetyUnknown
				return true, nil
			}
			if status == SMTSatisfiable {
				result.Outcome = SafetyCounterexample
				result.Depth = len(path.next)
				result.Invariant = invariant.name
				result.InitTransition = path.init
				result.NextTransitions = append([]int(nil), path.next...)
				result.States = states
				return true, nil
			}
		}
		for _, invariant := range inputs.traceInvariants {
			var states []DecodedState
			status, states, err = executor.CheckTraceAndDecode(ctx, invariant.parameter, invariant.expression,
				invariant.historyType, inputs.variableDeclarations)
			if err != nil {
				return false, fmt.Errorf("check trace invariant %s at depth %d: %w", invariant.name, len(path.next), err)
			}
			if status == SMTUnknown {
				result.Outcome = SafetyUnknown
				return true, nil
			}
			if status == SMTSatisfiable {
				result.Outcome = SafetyCounterexample
				result.Depth = len(path.next)
				result.Invariant = invariant.name
				result.TraceInvariant = true
				result.InitTransition = path.init
				result.NextTransitions = append([]int(nil), path.next...)
				result.States = states
				return true, nil
			}
		}
	}
	if len(path.next) == inputs.maxSteps && !inputs.checkDeadlocks {
		return false, nil
	}
	enabledNext := false
	for nextNumber := range inputs.nextTransitions {
		child := boundedPath{init: path.init, next: append(append([]int(nil), path.next...), nextNumber)}
		childExecutor, childErr := replayPendingNext(inputs, path, nextNumber)
		if childErr != nil {
			if errors.Is(childErr, errDisabledTransition) {
				continue
			}
			return false, childErr
		}
		childStatus, childErr := childExecutor.Check(ctx)
		if childErr != nil {
			return false, childErr
		}
		if childStatus == SMTUnknown {
			result.Outcome = SafetyUnknown
			return true, nil
		}
		if childStatus == SMTUnsatisfiable {
			continue
		}
		enabledNext = true
		for _, invariant := range inputs.actionInvariants {
			status, states, invariantErr := childExecutor.CheckPendingAndDecode(ctx, invariant.expression)
			if invariantErr != nil {
				return false, fmt.Errorf("check action invariant %s at depth %d: %w", invariant.name, len(child.next), invariantErr)
			}
			if status == SMTUnknown {
				result.Outcome = SafetyUnknown
				return true, nil
			}
			if status == SMTSatisfiable {
				result.Outcome = SafetyCounterexample
				result.Depth = len(child.next)
				result.Invariant = invariant.name
				result.ActionInvariant = true
				result.InitTransition = child.init
				result.NextTransitions = append([]int(nil), child.next...)
				result.States = states
				return true, nil
			}
		}
		if len(path.next) < inputs.maxSteps {
			*children = append(*children, child)
		}
	}
	if inputs.checkDeadlocks && !enabledNext {
		truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
		status, states, decodeErr := executor.CheckAndDecode(ctx, truth)
		if decodeErr != nil {
			return false, decodeErr
		}
		if status == SMTUnknown {
			result.Outcome = SafetyUnknown
			return true, nil
		}
		result.Outcome = SafetyDeadlock
		result.Depth = len(path.next)
		result.InitTransition = path.init
		result.NextTransitions = append([]int(nil), path.next...)
		result.States = states
		return true, nil
	}
	return false, nil
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
	return executor, nil
}

func replaySafetyPath(inputs modulePathInputs, path boundedPath) (*TransitionExecutor, error) {
	executor := NewTransitionExecutor(inputs.constants, inputs.variables)
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
