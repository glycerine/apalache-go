package apalache

import "fmt"

const (
	DefaultSimulationRuns = 100
	DefaultOutputTraces   = false
)

type InvariantCheckingMode uint8

const (
	InvariantBeforeJoin InvariantCheckingMode = iota
	InvariantAfterJoin
)

func (mode InvariantCheckingMode) String() string {
	switch mode {
	case InvariantBeforeJoin:
		return "before"
	case InvariantAfterJoin:
		return "after"
	default:
		return fmt.Sprintf("InvariantCheckingMode(%d)", mode)
	}
}

// ModelCheckerParams is the standalone counterpart of Scala's mutable
// checker parameter object. Derived collections are exposed defensively.
type ModelCheckerParams struct {
	CheckerInput *CheckerInput
	StepsBound   int
	Tuning       CheckerTuning

	DiscardDisabled   bool
	CheckForDeadlocks bool
	InvariantMode     InvariantCheckingMode
	MaxErrors         int
	TimeoutSMTSeconds int
	SMTEncoding       string

	TransitionFilter      string
	InvariantFilter       string
	JailTimeoutMinSeconds int64
	JailTimeoutFactor     int64
	IdleTimeoutSeconds    int64
	RandomSimulation      bool
	SimulationRuns        int
	SaveRuns              bool

	constants map[string]struct{}
	variables map[string]struct{}
}

func NewModelCheckerParams(
	input *CheckerInput,
	stepsBound int,
	tuningOptions map[string]string,
) (*ModelCheckerParams, error) {
	if input == nil || input.RootModule() == nil {
		return nil, fmt.Errorf("model checker parameters require checker input with a root module")
	}
	tuning, err := ParseCheckerTuning(tuningOptions)
	if err != nil {
		return nil, err
	}
	mode := InvariantBeforeJoin
	if tuning.invariantMode == "after" {
		mode = InvariantAfterJoin
	}
	minimum := tuning.jailTimeoutMinSec
	if _, configured := tuningOptions["search.split.timeout.minimum"]; !configured {
		minimum = 1800
	}
	factor := tuning.jailTimeoutFactor
	if _, configured := tuningOptions["search.split.timeout.factor"]; !configured {
		factor = 200
	}
	idle := tuning.idleTimeoutSec
	if _, configured := tuningOptions["search.idle.timeout"]; !configured {
		idle = 1800
	}
	params := &ModelCheckerParams{
		CheckerInput: input, StepsBound: stepsBound, Tuning: tuning,
		DiscardDisabled: true, CheckForDeadlocks: true, InvariantMode: mode,
		MaxErrors: 1, SMTEncoding: "OOPSLA19",
		TransitionFilter:      tuningOptions["search.transitionFilter"],
		InvariantFilter:       tuningOptions["search.invariantFilter"],
		JailTimeoutMinSeconds: minimum, JailTimeoutFactor: factor, IdleTimeoutSeconds: idle,
		RandomSimulation: tuning.simulation, SimulationRuns: tuning.SimulationRuns(), SaveRuns: tuning.OutputTraces(),
		constants: make(map[string]struct{}), variables: make(map[string]struct{}),
	}
	for _, declaration := range input.RootModule().ConstDeclarations() {
		params.constants[declaration.Name()] = struct{}{}
	}
	for _, declaration := range input.RootModule().VarDeclarations() {
		params.variables[declaration.Name()] = struct{}{}
	}
	return params, nil
}

func (params *ModelCheckerParams) Constants() map[string]struct{} {
	if params == nil {
		return nil
	}
	return cloneMap(params.constants)
}

func (params *ModelCheckerParams) Variables() map[string]struct{} {
	if params == nil {
		return nil
	}
	return cloneMap(params.variables)
}

func (params *ModelCheckerParams) IdleTimeoutMilliseconds() int64 {
	if params == nil {
		return 0
	}
	return params.IdleTimeoutSeconds * 1000
}

// ModelCheckerContext groups the parameters, normalized input, transition
// executor, and listeners used by a checker run.
type ModelCheckerContext struct {
	params    *ModelCheckerParams
	input     *CheckerInput
	executor  *TransitionExecutor
	listeners []ModelCheckerListener
}

func NewModelCheckerContext(
	params *ModelCheckerParams,
	input *CheckerInput,
	executor *TransitionExecutor,
	listeners []ModelCheckerListener,
) *ModelCheckerContext {
	return &ModelCheckerContext{
		params: params, input: input, executor: executor,
		listeners: append([]ModelCheckerListener(nil), listeners...),
	}
}

func (context *ModelCheckerContext) Params() *ModelCheckerParams             { return context.params }
func (context *ModelCheckerContext) CheckerInput() *CheckerInput             { return context.input }
func (context *ModelCheckerContext) TransitionExecutor() *TransitionExecutor { return context.executor }
func (context *ModelCheckerContext) Listeners() []ModelCheckerListener {
	return append([]ModelCheckerListener(nil), context.listeners...)
}
