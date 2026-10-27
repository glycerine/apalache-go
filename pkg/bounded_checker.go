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
)

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
	if module == nil {
		return BoundedSafetyResult{}, fmt.Errorf("cannot check a nil module")
	}
	if !module.HasProperty(PropertyTransitionsFound) {
		return BoundedSafetyResult{}, fmt.Errorf("bounded safety checking requires normalized transitions")
	}
	if maxSteps < 0 {
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
	if maxSteps > 0 && len(nextTransitions) == 0 {
		return BoundedSafetyResult{}, fmt.Errorf("module has no normalized next transitions")
	}
	invariants := make([]namedExpression, len(predicates.Invariants))
	for index, name := range predicates.Invariants {
		body, found := OperatorBody(module, name)
		if !found {
			return BoundedSafetyResult{}, fmt.Errorf("invariant %s was not found", name)
		}
		invariants[index] = namedExpression{name: name, expression: body}
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
			invariants: invariants, maxSteps: maxSteps,
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
	constants       []string
	variables       []string
	constantInit    Expr
	hasConstantInit bool
	assumptions     []*AssumeDecl
	initTransitions []Expr
	nextTransitions []Expr
	invariants      []namedExpression
	maxSteps        int
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
	if len(path.next) == inputs.maxSteps {
		return false, nil
	}
	for nextNumber := range inputs.nextTransitions {
		child := boundedPath{init: path.init, next: append(append([]int(nil), path.next...), nextNumber)}
		found, childErr := searchSafetyPath(ctx, inputs, child, result)
		if childErr != nil || found {
			return found, childErr
		}
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
	for depth, number := range path.next {
		if err := selectPathTransition(executor, number, inputs.nextTransitions, fmt.Sprintf("next at depth %d", depth+1)); err != nil {
			return nil, err
		}
		if err := executor.NextState(); err != nil {
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

func declarationNameSlice[T interface{ Name() string }](declarations []T) []string {
	result := make([]string, len(declarations))
	for index, declaration := range declarations {
		result[index] = declaration.Name()
	}
	return result
}
