package apalache

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
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

type SimulationReport struct {
	Runs            []SimulationResult
	Counterexamples []BoundedCounterexample
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
	MaxSteps    int
	MaxRuns     int
	Randomize   bool
	RandomSeed  int64
	SMTEncoding SMTEncoding
	Tuning      CheckerTuning
	MaxErrors   int
	RuleStats   *RuleStatListener
}

// SimulateBehaviorWithOptions finds one behavior while honoring the same
// transition filter syntax as the Scala checker.
func SimulateBehaviorWithOptions(
	ctx context.Context,
	module *Module,
	predicates DerivedPredicates,
	options SimulationOptions,
) (SimulationResult, error) {
	var random *rand.Rand
	if options.Randomize {
		seed := options.RandomSeed
		if seed == 0 {
			seed = time.Now().UnixNano()
		}
		random = rand.New(rand.NewSource(seed))
	}
	return simulateBehavior(ctx, module, predicates, options, random)
}

// SimulateBehaviorsWithOptions performs independent symbolic runs. Each run
// restores the constant state and randomly orders transition alternatives,
// matching Scala's random-simulation execution model.
func SimulateBehaviorsWithOptions(
	ctx context.Context,
	module *Module,
	predicates DerivedPredicates,
	options SimulationOptions,
) ([]SimulationResult, error) {
	if options.MaxRuns <= 0 {
		return nil, fmt.Errorf("maximum simulation run count must be positive")
	}
	seed := options.RandomSeed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	random := rand.New(rand.NewSource(seed))
	results := make([]SimulationResult, 0, options.MaxRuns)
	options.Randomize = true
	for run := 0; run < options.MaxRuns; run++ {
		result, err := simulateBehavior(ctx, module, predicates, options, random)
		if err != nil {
			return nil, fmt.Errorf("simulation run %d: %w", run+1, err)
		}
		results = append(results, result)
		if result.Outcome != SimulationCompleted {
			break
		}
	}
	return results, nil
}

// SimulateAndCheckWithOptions performs randomized symbolic runs and checks
// every selected path with the same state, action, and trace invariant engine
// used by exhaustive checking.
func SimulateAndCheckWithOptions(
	ctx context.Context,
	module *Module,
	predicates DerivedPredicates,
	options SimulationOptions,
) (SimulationReport, error) {
	if options.MaxRuns <= 0 {
		return SimulationReport{}, fmt.Errorf("maximum simulation run count must be positive")
	}
	if options.MaxErrors <= 0 {
		options.MaxErrors = 1
	}
	seed := options.RandomSeed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	random := rand.New(rand.NewSource(seed))
	report := SimulationReport{Runs: make([]SimulationResult, 0, options.MaxRuns)}
	options.Randomize = true
	for run := 0; run < options.MaxRuns; run++ {
		result, err := simulateBehavior(ctx, module, predicates, options, random)
		if err != nil {
			return SimulationReport{}, fmt.Errorf("simulation run %d: %w", run+1, err)
		}
		report.Runs = append(report.Runs, result)
		if result.Outcome != SimulationUnknown && len(predicates.Invariants) != 0 {
			remaining := options.MaxErrors - len(report.Counterexamples)
			checked, checkErr := CheckBoundedSafetyWithOptions(ctx, module, predicates, BoundedSafetyOptions{
				MaxSteps: len(result.NextTransitions), CheckDeadlocks: false, MaxErrors: remaining,
				SMTEncoding: options.SMTEncoding,
				Tuning:      options.Tuning.withExactTransitionPath(result.InitTransition, result.NextTransitions),
				RuleStats:   options.RuleStats,
			})
			if checkErr != nil {
				return SimulationReport{}, fmt.Errorf("check simulation run %d: %w", run+1, checkErr)
			}
			report.Counterexamples = append(report.Counterexamples, checked.Counterexamples...)
			if len(report.Counterexamples) >= options.MaxErrors {
				break
			}
		}
		if result.Outcome != SimulationCompleted {
			break
		}
	}
	return report, nil
}

func simulateBehavior(
	ctx context.Context,
	module *Module,
	predicates DerivedPredicates,
	options SimulationOptions,
	random *rand.Rand,
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
	smtEncoding := options.SMTEncoding
	if smtEncoding == "" {
		smtEncoding = SMTEncodingOOPSLA19
	}
	if smtEncoding != SMTEncodingOOPSLA19 && smtEncoding != SMTEncodingArrays && smtEncoding != SMTEncodingFunArrays {
		return SimulationResult{}, fmt.Errorf("unsupported simulator SMT encoding %s", smtEncoding)
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
		smtEncoding: smtEncoding,
		tuning:      options.Tuning,
		ruleStats:   options.RuleStats,
	}
	if len(inputs.initTransitions) == 0 {
		return SimulationResult{}, fmt.Errorf("module has no normalized initial transitions")
	}

	path := boundedPath{init: -1}
	for _, number := range simulationTransitionOrder(len(inputs.initTransitions), random) {
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
		for _, number := range simulationTransitionOrder(len(inputs.nextTransitions), random) {
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

func simulationTransitionOrder(count int, random *rand.Rand) []int {
	order := make([]int, count)
	for index := range order {
		order[index] = index
	}
	if random != nil {
		random.Shuffle(len(order), func(left, right int) {
			order[left], order[right] = order[right], order[left]
		})
	}
	return order
}
