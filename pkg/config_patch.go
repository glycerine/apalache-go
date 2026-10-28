package apalache

import (
	"os"
	"path/filepath"
)

type ConfigPatch interface{ configPatch() }

type RunContextPatch struct {
	Command    *string
	ConfigFile *string
}

func (RunContextPatch) configPatch() {}
func (patch RunContextPatch) Clone() RunContextPatch {
	patch.Command = cloneStringPointer(patch.Command)
	patch.ConfigFile = cloneStringPointer(patch.ConfigFile)
	return patch
}

type CommonPatch struct {
	OutDir            *string
	RunDir            *string
	Debug             *bool
	SMTProfiling      *bool
	WriteIntermediate *bool
	Profiling         *bool
	Features          *[]Feature
}

func (CommonPatch) configPatch() {}
func (patch CommonPatch) Clone() CommonPatch {
	patch.OutDir = cloneStringPointer(patch.OutDir)
	patch.RunDir = cloneStringPointer(patch.RunDir)
	patch.Debug = cloneBoolPointer(patch.Debug)
	patch.SMTProfiling = cloneBoolPointer(patch.SMTProfiling)
	patch.WriteIntermediate = cloneBoolPointer(patch.WriteIntermediate)
	patch.Profiling = cloneBoolPointer(patch.Profiling)
	patch.Features = cloneFeatureSlicePointer(patch.Features)
	return patch
}

type CheckerPatch struct {
	Tuning              *map[string]string
	Algorithm           *CheckerAlgorithm
	TLCConfig           *string
	DiscardDisabled     *bool
	ConstantInitializer *string
	Init                *string
	Invariants          *[]string
	Next                *string
	Length              *int
	MaxError            *int
	TimeoutSMTSeconds   *int
	CheckDeadlocks      *bool
	SMTSolver           *SMTSolverBackend
	SMTEncoding         *SMTEncoding
	TemporalProperties  *[]string
	View                *string
}

func (CheckerPatch) configPatch() {}
func (patch CheckerPatch) Clone() CheckerPatch {
	patch.Tuning = cloneStringMapPointer(patch.Tuning)
	patch.Algorithm = clonePointer(patch.Algorithm)
	patch.TLCConfig = cloneStringPointer(patch.TLCConfig)
	patch.DiscardDisabled = cloneBoolPointer(patch.DiscardDisabled)
	patch.ConstantInitializer = cloneStringPointer(patch.ConstantInitializer)
	patch.Init = cloneStringPointer(patch.Init)
	patch.Invariants = cloneStringSlicePointer(patch.Invariants)
	patch.Next = cloneStringPointer(patch.Next)
	patch.Length = clonePointer(patch.Length)
	patch.MaxError = clonePointer(patch.MaxError)
	patch.TimeoutSMTSeconds = clonePointer(patch.TimeoutSMTSeconds)
	patch.CheckDeadlocks = cloneBoolPointer(patch.CheckDeadlocks)
	patch.SMTSolver = clonePointer(patch.SMTSolver)
	patch.SMTEncoding = clonePointer(patch.SMTEncoding)
	patch.TemporalProperties = cloneStringSlicePointer(patch.TemporalProperties)
	patch.View = cloneStringPointer(patch.View)
	return patch
}

type TypecheckerPatch struct{ InferPoly *bool }

func (TypecheckerPatch) configPatch() {}
func (patch TypecheckerPatch) Clone() TypecheckerPatch {
	patch.InferPoly = cloneBoolPointer(patch.InferPoly)
	return patch
}

type TraceEvaluationPatch struct {
	Trace       InputSource
	Expressions *[]string
}

func (TraceEvaluationPatch) configPatch() {}
func (patch TraceEvaluationPatch) Clone() TraceEvaluationPatch {
	patch.Expressions = cloneStringSlicePointer(patch.Expressions)
	return patch
}

type ServerPatch struct {
	Port       *int
	ServerType *ServerType
}

func (ServerPatch) configPatch() {}
func (patch ServerPatch) Clone() ServerPatch {
	patch.Port = clonePointer(patch.Port)
	patch.ServerType = clonePointer(patch.ServerType)
	return patch
}

type ApalacheConfig struct {
	Context         RunContextPatch
	Common          CommonPatch
	Source          InputSource
	Output          *string
	Checker         CheckerPatch
	Typechecker     TypecheckerPatch
	TraceEvaluation TraceEvaluationPatch
	Server          ServerPatch
}

func (config ApalacheConfig) Clone() ApalacheConfig {
	config.Context = config.Context.Clone()
	config.Common = config.Common.Clone()
	config.Output = cloneStringPointer(config.Output)
	config.Checker = config.Checker.Clone()
	config.Typechecker = config.Typechecker.Clone()
	config.TraceEvaluation = config.TraceEvaluation.Clone()
	config.Server = config.Server.Clone()
	return config
}

func (config ApalacheConfig) WithCommand(command string) ApalacheConfig {
	config = config.Clone()
	config.Context.Command = &command
	return config
}

func (config ApalacheConfig) MergeWithLower(lower ApalacheConfig) ApalacheConfig {
	return ApalacheConfig{
		Context: RunContextPatch{
			Command:    choosePointer(config.Context.Command, lower.Context.Command),
			ConfigFile: choosePointer(config.Context.ConfigFile, lower.Context.ConfigFile),
		},
		Common: CommonPatch{
			OutDir:            choosePointer(config.Common.OutDir, lower.Common.OutDir),
			RunDir:            choosePointer(config.Common.RunDir, lower.Common.RunDir),
			Debug:             choosePointer(config.Common.Debug, lower.Common.Debug),
			SMTProfiling:      choosePointer(config.Common.SMTProfiling, lower.Common.SMTProfiling),
			WriteIntermediate: choosePointer(config.Common.WriteIntermediate, lower.Common.WriteIntermediate),
			Profiling:         choosePointer(config.Common.Profiling, lower.Common.Profiling),
			Features:          chooseFeatureSlice(config.Common.Features, lower.Common.Features),
		},
		Source: chooseInputSource(config.Source, lower.Source),
		Output: choosePointer(config.Output, lower.Output),
		Checker: CheckerPatch{
			Tuning:              mergeOptionalStringMaps(config.Checker.Tuning, lower.Checker.Tuning),
			Algorithm:           choosePointer(config.Checker.Algorithm, lower.Checker.Algorithm),
			TLCConfig:           choosePointer(config.Checker.TLCConfig, lower.Checker.TLCConfig),
			DiscardDisabled:     choosePointer(config.Checker.DiscardDisabled, lower.Checker.DiscardDisabled),
			ConstantInitializer: choosePointer(config.Checker.ConstantInitializer, lower.Checker.ConstantInitializer),
			Init:                choosePointer(config.Checker.Init, lower.Checker.Init),
			Invariants:          chooseStringSlice(config.Checker.Invariants, lower.Checker.Invariants),
			Next:                choosePointer(config.Checker.Next, lower.Checker.Next),
			Length:              choosePointer(config.Checker.Length, lower.Checker.Length),
			MaxError:            choosePointer(config.Checker.MaxError, lower.Checker.MaxError),
			TimeoutSMTSeconds:   choosePointer(config.Checker.TimeoutSMTSeconds, lower.Checker.TimeoutSMTSeconds),
			CheckDeadlocks:      choosePointer(config.Checker.CheckDeadlocks, lower.Checker.CheckDeadlocks),
			SMTSolver:           choosePointer(config.Checker.SMTSolver, lower.Checker.SMTSolver),
			SMTEncoding:         choosePointer(config.Checker.SMTEncoding, lower.Checker.SMTEncoding),
			TemporalProperties:  chooseStringSlice(config.Checker.TemporalProperties, lower.Checker.TemporalProperties),
			View:                choosePointer(config.Checker.View, lower.Checker.View),
		},
		Typechecker: TypecheckerPatch{InferPoly: choosePointer(config.Typechecker.InferPoly, lower.Typechecker.InferPoly)},
		TraceEvaluation: TraceEvaluationPatch{
			Trace:       chooseInputSource(config.TraceEvaluation.Trace, lower.TraceEvaluation.Trace),
			Expressions: chooseStringSlice(config.TraceEvaluation.Expressions, lower.TraceEvaluation.Expressions),
		},
		Server: ServerPatch{
			Port:       choosePointer(config.Server.Port, lower.Server.Port),
			ServerType: choosePointer(config.Server.ServerType, lower.Server.ServerType),
		},
	}
}

func (config ApalacheConfig) MergeWithDefaults() ApalacheConfig {
	return config.MergeWithLower(DefaultApalacheConfig())
}

func EmptyApalacheConfig() ApalacheConfig { return ApalacheConfig{} }

func DefaultApalacheConfig() ApalacheConfig {
	workingDirectory, err := os.Getwd()
	if err != nil {
		workingDirectory = "."
	}
	outDir := filepath.Join(workingDirectory, "_apalache-out")
	debug, smtprof, intermediate, profiling := false, false, false, false
	features := []Feature{}
	tuning := map[string]string{}
	algorithm, discard := AlgorithmIncremental, true
	length, maxError, timeout := 10, 1, 0
	solver, encoding := SMTSolverZ3, SMTEncodingOOPSLA19
	inferPoly := true
	port, serverType := 8822, ServerTypeChecker
	return ApalacheConfig{
		Common: CommonPatch{
			OutDir: &outDir, Debug: &debug, SMTProfiling: &smtprof, WriteIntermediate: &intermediate,
			Profiling: &profiling, Features: &features,
		},
		Checker: CheckerPatch{
			Tuning: &tuning, Algorithm: &algorithm, DiscardDisabled: &discard, Length: &length,
			MaxError: &maxError, TimeoutSMTSeconds: &timeout, SMTSolver: &solver, SMTEncoding: &encoding,
		},
		Typechecker: TypecheckerPatch{InferPoly: &inferPoly},
		Server:      ServerPatch{Port: &port, ServerType: &serverType},
	}
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneBoolPointer(value *bool) *bool { return clonePointer(value) }

func cloneStringSlicePointer(value *[]string) *[]string {
	if value == nil {
		return nil
	}
	clone := make([]string, len(*value))
	copy(clone, *value)
	return &clone
}

func cloneFeatureSlicePointer(value *[]Feature) *[]Feature {
	if value == nil {
		return nil
	}
	clone := make([]Feature, len(*value))
	copy(clone, *value)
	return &clone
}

func cloneStringMapPointer(value *map[string]string) *map[string]string {
	if value == nil {
		return nil
	}
	clone := cloneMap(*value)
	return &clone
}

func choosePointer[T any](higher, lower *T) *T {
	if higher != nil {
		return clonePointer(higher)
	}
	return clonePointer(lower)
}

func chooseStringSlice(higher, lower *[]string) *[]string {
	if higher != nil {
		return cloneStringSlicePointer(higher)
	}
	return cloneStringSlicePointer(lower)
}

func chooseFeatureSlice(higher, lower *[]Feature) *[]Feature {
	if higher != nil {
		return cloneFeatureSlicePointer(higher)
	}
	return cloneFeatureSlicePointer(lower)
}

func chooseInputSource(higher, lower InputSource) InputSource {
	if higher != nil {
		return higher
	}
	return lower
}

func mergeOptionalStringMaps(higher, lower *map[string]string) *map[string]string {
	if higher == nil {
		return cloneStringMapPointer(lower)
	}
	if lower == nil {
		return cloneStringMapPointer(higher)
	}
	merged := cloneMap(*lower)
	for key, value := range *higher {
		merged[key] = value
	}
	return &merged
}
