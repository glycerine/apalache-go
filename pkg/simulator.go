package apalache

import (
	"context"
	"errors"
	"fmt"
)

type SimulationOutcome uint8

const (
	SimulationUnknown SimulationOutcome = iota
	SimulationCompleted
	SimulationDeadlock
)

// SimulationResult is one deterministic symbolic behavior. Transition
// alternatives are considered in normalized declaration order.
type SimulationResult struct {
	Outcome         SimulationOutcome
	Depth           int
	InitTransition  int
	NextTransitions []int
	States          []DecodedState
}

// SimulateBehavior finds one satisfiable behavior through maxSteps
// applications of Next. It stops early when the selected behavior deadlocks.
func SimulateBehavior(
	ctx context.Context,
	module *Module,
	predicates DerivedPredicates,
	maxSteps int,
) (SimulationResult, error) {
	return SimulateBehaviorWithOptions(ctx, module, predicates, SimulationOptions{MaxSteps: maxSteps})
}

type SimulationOptions struct {
	MaxSteps int
	Tuning   CheckerTuning
}

// SimulateBehaviorWithOptions finds one behavior while honoring the same
// transition filter syntax as the Scala checker.
func SimulateBehaviorWithOptions(
	ctx context.Context,
	module *Module,
	predicates DerivedPredicates,
	options SimulationOptions,
) (SimulationResult, error) {
	maxSteps := options.MaxSteps
	if module == nil {
		return SimulationResult{}, fmt.Errorf("cannot simulate a nil module")
	}
	if !module.HasProperty(PropertyTransitionsFound) {
		return SimulationResult{}, fmt.Errorf("simulation requires normalized transitions")
	}
	if maxSteps < 0 {
		return SimulationResult{}, fmt.Errorf("maximum step count cannot be negative")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	stateConstraints, err := namedOperatorExpressions(module, predicates.StateConstraints, "state constraint")
	if err != nil {
		return SimulationResult{}, err
	}
	actionConstraints, err := namedOperatorExpressions(module, predicates.ActionConstraints, "action constraint")
	if err != nil {
		return SimulationResult{}, err
	}
	constantInit, hasConstantInit := OperatorBody(module, NormalizedConstInit)
	inputs := modulePathInputs{
		constants:    declarationNameSlice(module.ConstDeclarations()),
		variables:    declarationNameSlice(module.VarDeclarations()),
		constantInit: constantInit, hasConstantInit: hasConstantInit,
		assumptions:      module.AssumeDeclarations(),
		initTransitions:  TransitionsFromModule(module, NormalizedInitPrefix),
		nextTransitions:  TransitionsFromModule(module, NormalizedNextPrefix),
		stateConstraints: stateConstraints, actionConstraints: actionConstraints,
		tuning: options.Tuning,
	}
	if len(inputs.initTransitions) == 0 {
		return SimulationResult{}, fmt.Errorf("module has no normalized initial transitions")
	}

	path := boundedPath{init: -1}
	for number := range inputs.initTransitions {
		if !options.Tuning.allowsTransition(0, number) {
			continue
		}
		candidate := boundedPath{init: number}
		executor, replayErr := replaySafetyPath(inputs, candidate)
		if replayErr != nil {
			if errors.Is(replayErr, errDisabledTransition) {
				continue
			}
			return SimulationResult{}, replayErr
		}
		status, checkErr := executor.Check(ctx)
		if checkErr != nil {
			return SimulationResult{}, checkErr
		}
		if status == SMTUnknown {
			return SimulationResult{Outcome: SimulationUnknown, Depth: -1, InitTransition: -1}, nil
		}
		if status == SMTSatisfiable {
			path = candidate
			break
		}
	}
	if path.init < 0 {
		return SimulationResult{}, fmt.Errorf("no satisfiable initial state")
	}

	outcome := SimulationCompleted
	for len(path.next) < maxSteps {
		selected := -1
		for number := range inputs.nextTransitions {
			if !options.Tuning.allowsTransition(len(path.next)+1, number) {
				continue
			}
			executor, replayErr := replayPendingNext(inputs, path, number)
			if replayErr != nil {
				if errors.Is(replayErr, errDisabledTransition) {
					continue
				}
				return SimulationResult{}, replayErr
			}
			status, checkErr := executor.Check(ctx)
			if checkErr != nil {
				return SimulationResult{}, checkErr
			}
			if status == SMTUnknown {
				return SimulationResult{Outcome: SimulationUnknown, Depth: len(path.next), InitTransition: path.init,
					NextTransitions: append([]int(nil), path.next...)}, nil
			}
			if status == SMTSatisfiable {
				selected = number
				break
			}
		}
		if selected < 0 {
			outcome = SimulationDeadlock
			break
		}
		path.next = append(path.next, selected)
	}

	executor, err := replaySafetyPath(inputs, path)
	if err != nil {
		return SimulationResult{}, err
	}
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	status, states, err := executor.CheckAndDecode(ctx, truth)
	if err != nil {
		return SimulationResult{}, err
	}
	if status == SMTUnknown {
		outcome = SimulationUnknown
	} else if status != SMTSatisfiable {
		return SimulationResult{}, fmt.Errorf("selected simulation path became unsatisfiable")
	}
	return SimulationResult{
		Outcome: outcome, Depth: len(path.next), InitTransition: path.init,
		NextTransitions: append([]int(nil), path.next...), States: states,
	}, nil
}
