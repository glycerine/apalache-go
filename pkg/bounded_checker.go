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
	MaxSteps       int
	CheckDeadlocks bool
}

// BoundedSafetyResult describes either a completed bounded search or the
// first counterexample found in normalized transition order.
type BoundedSafetyResult struct {
	Outcome            SafetyOutcome
	Depth              int
	Invariant          string
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
	if ctx == nil {
		ctx = context.Background()
	}
	initTransitions := TransitionsFromModule(module, NormalizedInitPrefix)
	if len(initTransitions) == 0 {
		return BoundedSafetyResult{}, fmt.Errorf("module has no normalized initial transitions")
	}
	nextTransitions := TransitionsFromModule(module, NormalizedNextPrefix)
	invariants := make([]namedExpression, len(predicates.Invariants))
	for index, name := range predicates.Invariants {
		body, found := OperatorBody(module, name)
		if !found {
			return BoundedSafetyResult{}, fmt.Errorf("invariant %s was not found", name)
		}
		invariants[index] = namedExpression{name: name, expression: body}
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
	constantInit, hasConstantInit := OperatorBody(module, NormalizedConstInit)
	assumptions := module.AssumeDeclarations()

	result := BoundedSafetyResult{Outcome: SafetyNoCounterexample, Depth: -1, InitTransition: -1}
	for initNumber := range initTransitions {
		path := boundedPath{init: initNumber}
		found, err := searchSafetyPath(ctx, modulePathInputs{
			constants: constants, variables: variables, constantInit: constantInit,
			hasConstantInit: hasConstantInit, assumptions: assumptions,
			initTransitions: initTransitions, nextTransitions: nextTransitions,
			invariants: invariants, maxSteps: options.MaxSteps, checkDeadlocks: options.CheckDeadlocks,
			stateConstraints: stateConstraints, actionConstraints: actionConstraints,
		}, path, &result)
		if err != nil {
			return BoundedSafetyResult{}, err
		}
		if found {
			return result, nil
		}
	}
	return result, nil
}

type namedExpression struct {
	name       string
	expression Expr
}

type boundedPath struct {
	init int
	next []int
}

type modulePathInputs struct {
	constants         []string
	variables         []string
	constantInit      Expr
	hasConstantInit   bool
	assumptions       []*AssumeDecl
	initTransitions   []Expr
	nextTransitions   []Expr
	invariants        []namedExpression
	stateConstraints  []namedExpression
	actionConstraints []namedExpression
	maxSteps          int
	checkDeadlocks    bool
}

func searchSafetyPath(
	ctx context.Context,
	inputs modulePathInputs,
	path boundedPath,
	result *BoundedSafetyResult,
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
	for _, invariant := range inputs.invariants {
		violation := MustOperExpr(Op("NOT"), Typed(BoolT1), invariant.expression)
		var states []DecodedState
		status, states, err = executor.CheckAndDecode(ctx, violation)
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
	if len(path.next) == inputs.maxSteps && !inputs.checkDeadlocks {
		return false, nil
	}
	enabledNext := false
	for nextNumber := range inputs.nextTransitions {
		child := boundedPath{init: path.init, next: append(append([]int(nil), path.next...), nextNumber)}
		childExecutor, childErr := replaySafetyPath(inputs, child)
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
		if len(path.next) < inputs.maxSteps {
			found, searchErr := searchSafetyPath(ctx, inputs, child, result)
			if searchErr != nil || found {
				return found, searchErr
			}
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

func replaySafetyPath(inputs modulePathInputs, path boundedPath) (*TransitionExecutor, error) {
	executor := NewTransitionExecutor(inputs.constants, inputs.variables)
	if inputs.hasConstantInit {
		if err := executor.InitializeConstants(inputs.constantInit); err != nil {
			return nil, err
		}
	} else if len(inputs.constants) != 0 {
		return nil, fmt.Errorf("constants require a normalized constant initializer")
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
