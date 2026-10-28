package shai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	pal "github.com/glycerine/apalache-go"
	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var shaiFileWritingTuningKeys = map[string]struct{}{
	"search.outputTraces": {},
	"z3.dot_proof_file":   {}, "z3.trace": {}, "z3.trace_file_name": {},
	"z3.sat.drat.file": {}, "z3.sat.inprocess.out": {}, "z3.solver.axioms2files": {},
	"z3.solver.cancel_backup_file": {}, "z3.solver.proof.log": {}, "z3.solver.smtlib2_log": {},
	"z3.opt.dump_benchmarks": {}, "z3.opt.solution_prefix": {}, "z3.smt.arith.dump_lemmas": {},
}

type remoteSource struct {
	Kind    string   `json:"kind"`
	Content *string  `json:"content"`
	Aux     []string `json:"aux"`
	Format  string   `json:"format"`
}

type remoteCheckerConfig struct {
	Tuning             map[string]string `json:"tuning"`
	Algorithm          *string           `json:"algo"`
	TLCConfig          *string           `json:"config"`
	DiscardDisabled    *bool             `json:"discard-disabled"`
	ConstantInit       *string           `json:"cinit"`
	Init               *string           `json:"init"`
	Invariants         []string          `json:"inv"`
	Next               *string           `json:"next"`
	Length             *int              `json:"length"`
	MaxError           *int              `json:"max-error"`
	TimeoutSMT         *int              `json:"timeout-smt"`
	NoDeadlock         *bool             `json:"no-deadlock"`
	SMTSolver          *string           `json:"smt-solver"`
	SMTEncoding        *string           `json:"smt-encoding"`
	TemporalProperties []string          `json:"temporal"`
	View               *string           `json:"view"`
}

type remoteTypecheckerConfig struct {
	InferPoly *bool `json:"infer-poly"`
}

type remoteTraceConfig struct {
	Trace json.RawMessage `json:"trace"`
}

type remoteCommandConfig struct {
	Command           *string                 `json:"command"`
	ConfigFile        *string                 `json:"config-file"`
	OutDir            *string                 `json:"out-dir"`
	RunDir            *string                 `json:"run-dir"`
	Debug             *bool                   `json:"debug"`
	SMTProfile        *bool                   `json:"smtprof"`
	WriteIntermediate *bool                   `json:"write-intermediate"`
	Profiling         *bool                   `json:"profiling"`
	Features          []string                `json:"features"`
	Source            json.RawMessage         `json:"source"`
	Output            *string                 `json:"output"`
	Checker           remoteCheckerConfig     `json:"checker"`
	Typechecker       remoteTypecheckerConfig `json:"typechecker"`
	TraceEvaluation   remoteTraceConfig       `json:"tracee"`
	Server            json.RawMessage         `json:"server"`
	parsedSource      *remoteSource
}

type CmdExecutorService struct {
	cmd.UnimplementedCmdExecutorServer
}

func NewCmdExecutorService() *CmdExecutorService { return &CmdExecutorService{} }

func (*CmdExecutorService) Ping(context.Context, *cmd.PingRequest) (*cmd.PongResponse, error) {
	return &cmd.PongResponse{}, nil
}

func (*CmdExecutorService) Run(ctx context.Context, request *cmd.CmdRequest) (*cmd.CmdResponse, error) {
	switch request.GetCmd() {
	case cmd.Cmd_PARSE, cmd.Cmd_CHECK, cmd.Cmd_TYPECHECK, cmd.Cmd_TLA:
	default:
		return nil, status.Errorf(codes.InvalidArgument, "Invalid protobuf value for Cmd enum: %d", request.GetCmd())
	}
	config, err := parseRemoteCommandConfig(request.GetConfig())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if config.parsedSource == nil {
		return unexpectedCommandResponse(fmt.Errorf("Missing value for required option source")), nil
	}
	if format := strings.ToLower(config.parsedSource.Format); format != "" && format != "tla" {
		return unexpectedCommandResponse(fmt.Errorf("Go command executor does not support %s source format", format)), nil
	}
	path, cleanup, err := materializeShaiSources(config.parsedSource.content(), config.parsedSource.Aux)
	if err != nil {
		return unexpectedCommandResponse(err), nil
	}
	defer cleanup()

	switch request.GetCmd() {
	case cmd.Cmd_PARSE:
		return runRemoteParse(ctx, path)
	case cmd.Cmd_TYPECHECK:
		return runRemoteTypecheck(ctx, path, config)
	case cmd.Cmd_TLA:
		return runRemoteTLA(ctx, path, config.parsedSource.content())
	case cmd.Cmd_CHECK:
		if err = validateRemoteCheckerConfig(config.Checker); err != nil {
			return unexpectedCommandResponse(err), nil
		}
		return runRemoteCheck(ctx, path, config)
	default:
		panic("validated command became invalid")
	}
}

func validateRemoteCheckerConfig(checker remoteCheckerConfig) error {
	if _, err := pal.ParseCheckerTuning(checker.Tuning); err != nil {
		return fmt.Errorf("invalid checker tuning: %w", err)
	}
	if checker.Algorithm != nil && !strings.EqualFold(*checker.Algorithm, "incremental") {
		return fmt.Errorf("unsupported checker algorithm %q (Go port currently supports incremental)", *checker.Algorithm)
	}
	if checker.SMTEncoding != nil && !strings.EqualFold(*checker.SMTEncoding, "oopsla19") {
		return fmt.Errorf("unsupported SMT encoding %q (Go port currently supports oopsla19)", *checker.SMTEncoding)
	}
	if checker.SMTSolver != nil && !strings.EqualFold(*checker.SMTSolver, "z3") {
		return fmt.Errorf("unsupported SMT solver %q (Go port uses g3 v0.0.5 as its Z3 backend)", *checker.SMTSolver)
	}
	if checker.MaxError != nil && *checker.MaxError <= 0 {
		return fmt.Errorf("checker.max-error must be positive")
	}
	if checker.MaxError != nil && *checker.MaxError > 1 && checker.View == nil {
		return fmt.Errorf("checker.max-error=%d requires checker.view", *checker.MaxError)
	}
	if checker.Length != nil && *checker.Length < 0 {
		return fmt.Errorf("checker.length cannot be negative")
	}
	if checker.TimeoutSMT != nil && *checker.TimeoutSMT < 0 {
		return fmt.Errorf("checker.timeout-smt cannot be negative")
	}
	if checker.TimeoutSMT != nil && int64(*checker.TimeoutSMT) > int64(time.Duration(1<<63-1)/time.Second) {
		return fmt.Errorf("checker.timeout-smt is too large")
	}
	if (checker.Init == nil) != (checker.Next == nil) {
		return fmt.Errorf("checker.init and checker.next must be provided together")
	}
	return nil
}

func (source *remoteSource) content() string {
	if source == nil || source.Content == nil {
		return ""
	}
	return *source.Content
}

func parseRemoteCommandConfig(document string) (remoteCommandConfig, error) {
	var config remoteCommandConfig
	if err := decodeStrictJSON([]byte(document), &config); err != nil {
		return config, fmt.Errorf("invalid remote configuration: %w", err)
	}
	paths := []struct {
		present bool
		path    string
	}{
		{config.ConfigFile != nil, "$.config-file"}, {config.OutDir != nil, "$.out-dir"},
		{config.RunDir != nil, "$.run-dir"}, {config.Output != nil, "$.output"},
		{config.Checker.TLCConfig != nil, "$.checker.config"},
	}
	for _, path := range paths {
		if path.present {
			return config, fmt.Errorf("%s: Filesystem paths are not allowed in remote configuration", path.path)
		}
	}
	for key := range config.Checker.Tuning {
		if _, forbidden := shaiFileWritingTuningKeys[key]; forbidden {
			return config, fmt.Errorf("$.checker.tuning.%s: This tuning option can write files and is not allowed in remote configuration", key)
		}
	}
	if len(config.TraceEvaluation.Trace) != 0 && string(config.TraceEvaluation.Trace) != "null" {
		var trace remoteSource
		if err := decodeStrictJSON(config.TraceEvaluation.Trace, &trace); err != nil || strings.EqualFold(trace.Kind, "file") {
			return config, fmt.Errorf("$.tracee.trace: File-backed sources are not allowed in remote configuration; provide in-memory content")
		}
	}
	if len(config.Source) == 0 || string(config.Source) == "null" {
		return config, nil
	}
	var filePath string
	if json.Unmarshal(config.Source, &filePath) == nil {
		return config, fmt.Errorf("$.source: File-backed sources are not allowed in remote configuration; provide in-memory content")
	}
	var source remoteSource
	if err := decodeStrictJSON(config.Source, &source); err != nil {
		return config, fmt.Errorf("$.source: %w", err)
	}
	if strings.EqualFold(source.Kind, "file") {
		return config, fmt.Errorf("$.source: File-backed sources are not allowed in remote configuration; provide in-memory content")
	}
	if source.Kind != "" && !strings.EqualFold(source.Kind, "string") {
		return config, fmt.Errorf("$.source.kind: expected string")
	}
	if source.Content == nil {
		return config, fmt.Errorf("$.source.content: Missing value for required option content")
	}
	config.parsedSource = &source
	return config, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func runRemoteParse(ctx context.Context, path string) (*cmd.CmdResponse, error) {
	result := pal.ParseFile(ctx, path, pal.FrontendOptions{})
	if !result.OK() {
		return passFailureCommandResponse("Parser", result.Diagnostics), nil
	}
	return successfulCommandResponse(string(result.IR)), nil
}

func runRemoteTypecheck(ctx context.Context, path string, config remoteCommandConfig) (*cmd.CmdResponse, error) {
	result := pal.ParseAndTypeCheckFile(ctx, path, pal.FrontendOptions{}, remoteTypecheckOptions(config))
	if !result.OK() {
		return passFailureCommandResponse("TypeCheckerSnowcat", result.Diagnostics), nil
	}
	encoded, err := pal.MarshalSingleModuleIRJSON(result.Native, pal.IRJSONOptions{})
	if err != nil {
		return unexpectedCommandResponse(err), nil
	}
	return successfulCommandResponse(string(encoded)), nil
}

func runRemoteTLA(ctx context.Context, path, source string) (*cmd.CmdResponse, error) {
	result := pal.ParseFile(ctx, path, pal.FrontendOptions{IncludeSource: true})
	if !result.OK() {
		return passFailureCommandResponse("Parser", result.Diagnostics), nil
	}
	annotations := pal.ExtractTypeAnnotations(path, source, result.Native, result.Sources)
	formatted, err := pal.FormatTLAModule(result.Native, pal.TLAFormatOptions{
		ExtendedModules: []string{"Integers", "Sequences", "FiniteSets", "TLC", "Apalache", "Variants"},
		Annotations:     annotations.Annotations,
	})
	if err != nil {
		return unexpectedCommandResponse(err), nil
	}
	encoded, _ := json.Marshal(formatted)
	return successfulCommandResponse(string(encoded)), nil
}

func runRemoteCheck(ctx context.Context, path string, config remoteCommandConfig) (*cmd.CmdResponse, error) {
	if config.Checker.TimeoutSMT != nil {
		ctx = pal.WithSMTQueryTimeout(ctx, time.Duration(*config.Checker.TimeoutSMT)*time.Second)
	}
	frontend := pal.ParseAndTypeCheckFile(ctx, path, pal.FrontendOptions{}, remoteTypecheckOptions(config))
	if !frontend.OK() {
		return passFailureCommandResponse("TypeCheckerSnowcat", frontend.Diagnostics), nil
	}
	checker := config.Checker
	init, next := "Init", "Next"
	if checker.Init != nil {
		init = *checker.Init
	}
	if checker.Next != nil {
		next = *checker.Next
	}
	prepared, err := pal.PrepareSafetyTransitions(frontend.Native, pal.SafetyPipelineOptions{
		Specification: pal.SpecificationConfiguration{
			Behavior: pal.InitNextSpec{Init: init, Next: next}, ConstantInit: valueOrEmpty(checker.ConstantInit),
			Invariants: checker.Invariants, TemporalProperties: checker.TemporalProperties, View: valueOrEmpty(checker.View),
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		return passFailureCommandResponse("ConfigurationPass", map[string]string{"msg": err.Error()}), nil
	}
	length := 10
	if checker.Length != nil {
		length = *checker.Length
	}
	checkDeadlocks := checker.NoDeadlock == nil || !*checker.NoDeadlock
	maxErrors := 1
	if checker.MaxError != nil {
		maxErrors = *checker.MaxError
	}
	tuning, err := pal.ParseCheckerTuning(checker.Tuning)
	if err != nil {
		return unexpectedCommandResponse(err), nil
	}
	discardDisabled := true
	if checker.DiscardDisabled != nil {
		discardDisabled = *checker.DiscardDisabled
	}
	result, err := pal.CheckBoundedSafetyWithOptions(ctx, prepared.Module, prepared.Predicates, pal.BoundedSafetyOptions{
		MaxSteps: length, CheckDeadlocks: checkDeadlocks, Tuning: tuning, MaxErrors: maxErrors,
		DiscardDisabled: &discardDisabled,
	})
	if err != nil {
		return passFailureCommandResponse("BoundedChecker", map[string]string{"msg": err.Error()}), nil
	}
	if result.Outcome == pal.SafetyCounterexample || result.Outcome == pal.SafetyDeadlock {
		checkingResult := "violation"
		if result.Outcome == pal.SafetyDeadlock {
			checkingResult = "deadlock"
		}
		counterexamples := make([]any, 0, max(1, len(result.Counterexamples)))
		if result.Outcome == pal.SafetyDeadlock {
			counterexamples = append(counterexamples, encodedRemoteCounterexample(prepared.Module, result.Depth, result.States))
		} else {
			for _, counterexample := range result.Counterexamples {
				counterexamples = append(counterexamples,
					encodedRemoteCounterexample(prepared.Module, counterexample.Depth, counterexample.States))
			}
		}
		return passFailureCommandResponse("BoundedChecker", map[string]any{
			"checking_result": checkingResult, "counterexamples": counterexamples,
		}), nil
	}
	encoded, err := pal.MarshalSingleModuleIRJSON(prepared.Module, pal.IRJSONOptions{})
	if err != nil {
		return unexpectedCommandResponse(err), nil
	}
	return successfulCommandResponse(string(encoded)), nil
}

func encodedRemoteCounterexample(module *pal.Module, depth int, states []pal.DecodedState) any {
	counterexample := any(map[string]any{"depth": depth})
	if trace, err := pal.MarshalDecodedTraceITF(module, states, pal.ITFOptions{}); err == nil {
		_ = json.Unmarshal(trace, &counterexample)
	}
	return counterexample
}

func remoteTypecheckOptions(config remoteCommandConfig) pal.TypeCheckOptions {
	options := pal.TypeCheckOptions{}
	if config.Typechecker.InferPoly != nil {
		options.DisablePolytypes = !*config.Typechecker.InferPoly
	}
	for _, feature := range config.Features {
		options.UseRows = options.UseRows || feature == "rows"
	}
	return options
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func successfulCommandResponse(data string) *cmd.CmdResponse {
	return &cmd.CmdResponse{Result: &cmd.CmdResponse_Success{Success: data}}
}

func passFailureCommandResponse(pass string, errorData any) *cmd.CmdResponse {
	data, _ := json.Marshal(map[string]any{"pass_name": pass, "error_data": errorData})
	return &cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{
		ErrorType: cmd.CmdErrorType_PASS_FAILURE, Data: string(data),
	}}}
}

func unexpectedCommandResponse(err error) *cmd.CmdResponse {
	data, _ := json.Marshal(map[string]any{"msg": err.Error(), "stack_trace": []string{}})
	return &cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{
		ErrorType: cmd.CmdErrorType_UNEXPECTED, Data: string(data),
	}}}
}
