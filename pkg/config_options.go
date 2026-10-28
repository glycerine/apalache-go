package apalache

// ConfigParseResult stores expected configuration failures as values. The
// contained value is intentionally private so success cannot be inferred from
// a zero value of T.
type ConfigParseResult[T any] struct {
	value    T
	hasValue bool
	errors   []string
	warnings []string
}

func ConfigSuccess[T any](value T, warnings ...string) ConfigParseResult[T] {
	return ConfigParseResult[T]{value: value, hasValue: true, warnings: append([]string(nil), warnings...)}
}

func ConfigFailure[T any](errors []string, warnings ...string) ConfigParseResult[T] {
	return ConfigParseResult[T]{errors: append([]string(nil), errors...), warnings: append([]string(nil), warnings...)}
}

func ConfigFailureMessage[T any](message string) ConfigParseResult[T] {
	return ConfigFailure[T]([]string{message})
}

func ConfigFailureFrom[T, U any](result ConfigParseResult[U]) ConfigParseResult[T] {
	return ConfigFailure[T](result.Errors(), result.Warnings()...)
}

func ConfigWithWarnings[T any](result ConfigParseResult[T], warnings ...string) ConfigParseResult[T] {
	result.errors = append([]string(nil), result.errors...)
	result.warnings = append(append([]string(nil), result.warnings...), warnings...)
	return result
}

func (result ConfigParseResult[T]) IsSuccess() bool {
	return result.hasValue && len(result.errors) == 0
}
func (result ConfigParseResult[T]) Value() (T, bool) { return result.value, result.hasValue }
func (result ConfigParseResult[T]) Errors() []string { return append([]string(nil), result.errors...) }
func (result ConfigParseResult[T]) Warnings() []string {
	return append([]string(nil), result.warnings...)
}
func (result ConfigParseResult[T]) RequireValue() T {
	if !result.IsSuccess() {
		panic("Configuration parse result has no value")
	}
	return result.value
}

type ValidatedOptions interface{ validatedOptions() }

type CommonOptions struct {
	Debug             bool
	Features          []Feature
	OutDir            string
	Profiling         bool
	RunDir            *string
	SMTProfiling      bool
	WriteIntermediate bool
}

func (CommonOptions) validatedOptions() {}
func (options CommonOptions) Clone() CommonOptions {
	options.Features = append([]Feature(nil), options.Features...)
	options.RunDir = cloneStringPointer(options.RunDir)
	return options
}

type CommandInitializationOptions struct {
	Command string
	Common  CommonOptions
	Source  InputSource
}

func (CommandInitializationOptions) validatedOptions() {}
func (options CommandInitializationOptions) Clone() CommandInitializationOptions {
	options.Common = options.Common.Clone()
	return options
}

type ModuleIOOptions struct {
	Source InputSource
	Output *string
}

func (ModuleIOOptions) validatedOptions() {}
func (options ModuleIOOptions) Clone() ModuleIOOptions {
	options.Output = cloneStringPointer(options.Output)
	return options
}

type TypecheckerOptions struct{ InferPoly bool }

func (TypecheckerOptions) validatedOptions() {}

type CheckerOptions struct {
	Algorithm         CheckerAlgorithm
	DiscardDisabled   bool
	Length            int
	MaxError          int
	TimeoutSMTSeconds int
	CheckDeadlocks    bool
	SMTSolver         SMTSolverBackend
	SMTEncoding       SMTEncoding
	Tuning            map[string]string
}

func (CheckerOptions) validatedOptions() {}
func (options CheckerOptions) Clone() CheckerOptions {
	options.Tuning = cloneMap(options.Tuning)
	return options
}

type TLCConfigInput struct {
	Config TLCConfig
	Path   string
}

func (TLCConfigInput) validatedOptions() {}
func (input TLCConfigInput) Clone() TLCConfigInput {
	input.Config = cloneTLCConfig(input.Config)
	return input
}

type SpecificationOptions struct {
	BehaviorSpec        BehaviorSpec
	ConstantInitializer *string
	Invariants          []string
	TemporalProperties  []string
	TLCConfig           *TLCConfigInput
	View                *string
	Persistent          []string
}

func (SpecificationOptions) validatedOptions() {}
func (options SpecificationOptions) Clone() SpecificationOptions {
	options.ConstantInitializer = cloneStringPointer(options.ConstantInitializer)
	options.Invariants = append([]string(nil), options.Invariants...)
	options.TemporalProperties = append([]string(nil), options.TemporalProperties...)
	if options.TLCConfig != nil {
		clone := options.TLCConfig.Clone()
		options.TLCConfig = &clone
	}
	options.View = cloneStringPointer(options.View)
	options.Persistent = append([]string(nil), options.Persistent...)
	return options
}

type TraceEvaluationOptions struct {
	Trace       InputSource
	Expressions []string
}

func (TraceEvaluationOptions) validatedOptions() {}
func (options TraceEvaluationOptions) Clone() TraceEvaluationOptions {
	options.Expressions = append([]string(nil), options.Expressions...)
	return options
}

type ServerOptions struct {
	Port       int
	ServerType ServerType
}

func (ServerOptions) validatedOptions() {}

type ValidatedParseOptions struct {
	Common CommonOptions
	Source InputSource
	Output *string
}

func (ValidatedParseOptions) validatedOptions() {}
func (options ValidatedParseOptions) Clone() ValidatedParseOptions {
	options.Common = options.Common.Clone()
	options.Output = cloneStringPointer(options.Output)
	return options
}

type ValidatedTypecheckOptions struct {
	Common      CommonOptions
	Source      InputSource
	Output      *string
	Typechecker TypecheckerOptions
}

func (ValidatedTypecheckOptions) validatedOptions() {}
func (options ValidatedTypecheckOptions) Clone() ValidatedTypecheckOptions {
	options.Common = options.Common.Clone()
	options.Output = cloneStringPointer(options.Output)
	return options
}

type ValidatedCheckOptions struct {
	Common        CommonOptions
	Source        InputSource
	Output        *string
	Typechecker   TypecheckerOptions
	Checker       CheckerOptions
	Specification SpecificationOptions
}

func (ValidatedCheckOptions) validatedOptions() {}
func (options ValidatedCheckOptions) Clone() ValidatedCheckOptions {
	options.Common = options.Common.Clone()
	options.Output = cloneStringPointer(options.Output)
	options.Checker = options.Checker.Clone()
	options.Specification = options.Specification.Clone()
	return options
}

type ValidatedTraceOptions struct {
	Common          CommonOptions
	Source          InputSource
	Output          *string
	Typechecker     TypecheckerOptions
	Checker         CheckerOptions
	Specification   SpecificationOptions
	TraceEvaluation TraceEvaluationOptions
}

func (ValidatedTraceOptions) validatedOptions() {}
func (options ValidatedTraceOptions) Clone() ValidatedTraceOptions {
	options.Common = options.Common.Clone()
	options.Output = cloneStringPointer(options.Output)
	options.Checker = options.Checker.Clone()
	options.Specification = options.Specification.Clone()
	options.TraceEvaluation = options.TraceEvaluation.Clone()
	return options
}
func (options ValidatedTraceOptions) WithLength(length int) ValidatedTraceOptions {
	options = options.Clone()
	options.Checker.Length = length
	return options
}

type ValidatedServerOptions struct {
	Common CommonOptions
	Server ServerOptions
}

func (ValidatedServerOptions) validatedOptions() {}
func (options ValidatedServerOptions) Clone() ValidatedServerOptions {
	options.Common = options.Common.Clone()
	return options
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneTLCConfig(config TLCConfig) TLCConfig {
	config.ConstAssignments = cloneMap(config.ConstAssignments)
	config.ConstAssignmentOrder = append([]string(nil), config.ConstAssignmentOrder...)
	config.ConstReplacements = cloneMap(config.ConstReplacements)
	config.ConstReplacementOrder = append([]string(nil), config.ConstReplacementOrder...)
	config.StateConstraints = append([]string(nil), config.StateConstraints...)
	config.ActionConstraints = append([]string(nil), config.ActionConstraints...)
	config.Invariants = append([]string(nil), config.Invariants...)
	config.TemporalProperties = append([]string(nil), config.TemporalProperties...)
	if config.CheckDeadlock != nil {
		value := *config.CheckDeadlock
		config.CheckDeadlock = &value
	}
	return config
}
