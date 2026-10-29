package apalache

import (
	"context"
	"encoding/json"
	"fmt"
)

type CheckerResultKind uint8

const (
	CheckerNoError CheckerResultKind = iota
	CheckerError
	CheckerDeadlock
	CheckerExecutionsTooShort
	CheckerSMTTimeout
	CheckerInterrupted
	CheckerRuntimeError
)

func (kind CheckerResultKind) String() string {
	switch kind {
	case CheckerNoError:
		return "NoError"
	case CheckerError:
		return "Error"
	case CheckerDeadlock:
		return "Deadlock"
	case CheckerExecutionsTooShort:
		return "ExecutionsTooShort"
	case CheckerSMTTimeout:
		return "SmtTimeout"
	case CheckerInterrupted:
		return "Interrupted"
	case CheckerRuntimeError:
		return "RuntimeError"
	default:
		return fmt.Sprintf("CheckerResultKind(%d)", kind)
	}
}

func (kind CheckerResultKind) IsOK() bool {
	return kind == CheckerNoError || kind == CheckerExecutionsTooShort || kind == CheckerSMTTimeout
}

// CheckerResult is the Go sum-type representation of Scala's CheckerResult.
type CheckerResult struct {
	Kind            CheckerResultKind
	NErrors         int
	NTimeouts       int
	Counterexamples []BoundedCounterexample
	Deadlock        *BoundedCounterexample
}

func (result CheckerResult) String() string { return result.Kind.String() }
func (result CheckerResult) IsOK() bool     { return result.Kind.IsOK() }

// MarshalJSON matches Checker.CheckerResult's ujson wire representation.
func (result CheckerResult) MarshalJSON() ([]byte, error) {
	root := map[string]any{"checking_result": result.String()}
	switch result.Kind {
	case CheckerError:
		counterexamples, err := marshalCheckerCounterexamples(result.Counterexamples)
		if err != nil {
			return nil, err
		}
		root["counterexamples"] = counterexamples
		root["nerrors"] = result.NErrors
	case CheckerDeadlock:
		counterexamples := make([]json.RawMessage, 0, 1)
		if result.Deadlock != nil {
			encoded, err := marshalCheckerCounterexample(*result.Deadlock)
			if err != nil {
				return nil, err
			}
			counterexamples = append(counterexamples, encoded)
		}
		root["counterexamples"] = counterexamples
	case CheckerSMTTimeout:
		root["ntimeouts"] = result.NTimeouts
	}
	return json.Marshal(root)
}

func marshalCheckerCounterexamples(counterexamples []BoundedCounterexample) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, len(counterexamples))
	for index, counterexample := range counterexamples {
		encoded, err := marshalCheckerCounterexample(counterexample)
		if err != nil {
			return nil, fmt.Errorf("encode counterexample %d: %w", index, err)
		}
		result[index] = encoded
	}
	return result, nil
}

func marshalCheckerCounterexample(counterexample BoundedCounterexample) (json.RawMessage, error) {
	if counterexample.Module == nil {
		return nil, fmt.Errorf("counterexample has no root module")
	}
	encoded, err := MarshalDecodedTraceITF(counterexample.Module, counterexample.States, ITFOptions{})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

// Checker is the standalone, cancellable Go model-checker boundary.
type Checker interface {
	Run(context.Context) (CheckerResult, error)
}

type BoundedModelChecker struct {
	Module     *Module
	Predicates DerivedPredicates
	Options    BoundedSafetyOptions
}

func (checker BoundedModelChecker) Run(ctx context.Context) (CheckerResult, error) {
	result, err := CheckBoundedSafetyWithOptions(ctx, checker.Module, checker.Predicates, checker.Options)
	if err != nil {
		return CheckerResult{}, err
	}
	return result.AsCheckerResult(checker.Module), nil
}

func (result BoundedSafetyResult) AsCheckerResult(module *Module) CheckerResult {
	switch result.Outcome {
	case SafetyNoCounterexample:
		return CheckerResult{Kind: CheckerNoError}
	case SafetyExecutionsTooShort:
		return CheckerResult{Kind: CheckerExecutionsTooShort}
	case SafetyCounterexample:
		counterexamples := cloneCounterexamples(result.Counterexamples)
		for index := range counterexamples {
			if counterexamples[index].Module == nil {
				counterexamples[index].Module = module
			}
		}
		return CheckerResult{Kind: CheckerError, NErrors: len(counterexamples), Counterexamples: counterexamples}
	case SafetyDeadlock:
		if len(result.States) == 0 {
			return CheckerResult{Kind: CheckerDeadlock}
		}
		deadlock := BoundedCounterexample{
			Module: module, Depth: result.Depth, Deadlock: true, InitTransition: result.InitTransition,
			NextTransitions: append([]int(nil), result.NextTransitions...), States: cloneDecodedStates(result.States),
		}
		return CheckerResult{Kind: CheckerDeadlock, Deadlock: &deadlock}
	default:
		return CheckerResult{Kind: CheckerSMTTimeout, NTimeouts: 1}
	}
}

var _ Checker = BoundedModelChecker{}

func NewCheckerError(counterexamples ...BoundedCounterexample) CheckerResult {
	return CheckerResult{Kind: CheckerError, NErrors: 1, Counterexamples: cloneCounterexamples(counterexamples)}
}

func cloneCheckerResult(result CheckerResult) CheckerResult {
	result.Counterexamples = cloneCounterexamples(result.Counterexamples)
	if result.Deadlock != nil {
		copy := cloneBoundedCounterexample(*result.Deadlock)
		result.Deadlock = &copy
	}
	return result
}

func cloneCounterexamples(counterexamples []BoundedCounterexample) []BoundedCounterexample {
	result := make([]BoundedCounterexample, len(counterexamples))
	for index, counterexample := range counterexamples {
		result[index] = cloneBoundedCounterexample(counterexample)
	}
	return result
}

// SearchState accumulates checker outcomes across verification conditions and
// random simulation runs.
type SearchState struct {
	params          *ModelCheckerParams
	result          CheckerResult
	foundErrors     int
	timeouts        int
	counterexamples []BoundedCounterexample
	runsLeft        int
}

func NewSearchState(params *ModelCheckerParams) *SearchState {
	runs := 1
	if params != nil && params.RandomSimulation {
		runs = params.SimulationRuns
	}
	return &SearchState{params: params, result: CheckerResult{Kind: CheckerNoError}, runsLeft: runs}
}

func (state *SearchState) FoundErrors() int { return state.foundErrors }
func (state *SearchState) Timeouts() int    { return state.timeouts }
func (state *SearchState) RunsLeft() int    { return state.runsLeft }

func (state *SearchState) FinalResult() CheckerResult {
	if state.result.Kind != CheckerNoError {
		return cloneCheckerResult(state.result)
	}
	if state.foundErrors > 0 {
		return CheckerResult{
			Kind: CheckerError, NErrors: state.foundErrors,
			Counterexamples: cloneCounterexamples(state.counterexamples),
		}
	}
	if state.timeouts > 0 {
		return CheckerResult{Kind: CheckerSMTTimeout, NTimeouts: state.timeouts}
	}
	return CheckerResult{Kind: CheckerNoError}
}

func (state *SearchState) CanContinue() bool {
	return state.result.Kind == CheckerNoError && state.runsLeft > 0
}

func (state *SearchState) OnResult(result CheckerResult) {
	switch result.Kind {
	case CheckerError:
		state.foundErrors++
		state.counterexamples = append(state.counterexamples, cloneCounterexamples(result.Counterexamples)...)
		maxErrors := 1
		if state.params != nil {
			maxErrors = state.params.MaxErrors
		}
		if state.foundErrors >= maxErrors {
			state.result = CheckerResult{
				Kind: CheckerError, NErrors: state.foundErrors,
				Counterexamples: cloneCounterexamples(state.counterexamples),
			}
		} else {
			state.result = CheckerResult{Kind: CheckerNoError}
		}
	case CheckerDeadlock:
		if state.foundErrors > 0 {
			state.result = CheckerResult{
				Kind: CheckerError, NErrors: state.foundErrors,
				Counterexamples: cloneCounterexamples(state.counterexamples),
			}
		} else {
			state.result = cloneCheckerResult(result)
		}
	case CheckerSMTTimeout:
		state.timeouts += result.NTimeouts
		state.result = CheckerResult{Kind: CheckerNoError}
	default:
		state.result = cloneCheckerResult(result)
	}
}

func (state *SearchState) OnRunDone() {
	state.runsLeft--
	if state.runsLeft > 0 && state.result.Kind == CheckerExecutionsTooShort {
		state.result = CheckerResult{Kind: CheckerNoError}
	}
}
