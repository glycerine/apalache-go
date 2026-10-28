package apalache

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	DefaultInitOperator   = "Init"
	DefaultNextOperator   = "Next"
	DefaultCheckDeadlocks = true
)

func ResolveCommandInitialization(config ApalacheConfig) ConfigParseResult[CommandInitializationOptions] {
	command := requireConfigCommand(config)
	if !command.IsSuccess() {
		return ConfigFailureFrom[CommandInitializationOptions](command)
	}
	return ConfigSuccess(CommandInitializationOptions{
		Command: command.RequireValue(), Common: resolveCommonOptions(config), Source: config.Source,
	}, command.Warnings()...)
}

func ResolveParseOptions(config ApalacheConfig) ConfigParseResult[ValidatedParseOptions] {
	command := requireConfigCommand(config)
	if !command.IsSuccess() {
		return ConfigFailureFrom[ValidatedParseOptions](command)
	}
	if config.Source == nil {
		return ConfigFailure[ValidatedParseOptions]([]string{"Missing value for required option " + ConfigSource}, command.Warnings()...)
	}
	return ConfigSuccess(ValidatedParseOptions{
		Common: resolveCommonOptions(config), Source: config.Source, Output: cloneStringPointer(config.Output),
	}, command.Warnings()...)
}

func ResolveTypecheckOptions(config ApalacheConfig) ConfigParseResult[ValidatedTypecheckOptions] {
	parsed := ResolveParseOptions(config)
	if !parsed.IsSuccess() {
		return ConfigFailureFrom[ValidatedTypecheckOptions](parsed)
	}
	base := parsed.RequireValue()
	typechecker := config.MergeWithDefaults().Typechecker
	return ConfigSuccess(ValidatedTypecheckOptions{
		Common: base.Common, Source: base.Source, Output: base.Output,
		Typechecker: TypecheckerOptions{InferPoly: requireConfigDefault(typechecker.InferPoly, ConfigTypechecker+"."+ConfigInferPoly)},
	}, parsed.Warnings()...)
}

func ResolveCheckOptions(config ApalacheConfig) ConfigParseResult[ValidatedCheckOptions] {
	typechecked := ResolveTypecheckOptions(config)
	if !typechecked.IsSuccess() {
		return ConfigFailureFrom[ValidatedCheckOptions](typechecked)
	}
	errorsFound := make([]string, 0)
	warnings := typechecked.Warnings()
	configured := config.Checker
	withDefaults := config.MergeWithDefaults().Checker
	solver := requireConfigDefault(withDefaults.SMTSolver, ConfigChecker+"."+ConfigSMTSolver)
	encoding := requireConfigDefault(withDefaults.SMTEncoding, ConfigChecker+"."+ConfigSMTEncoding)
	maxError := requireConfigDefault(withDefaults.MaxError, ConfigChecker+"."+ConfigMaxError)
	if solver == SMTSolverCVC5 && encoding != SMTEncodingOOPSLA19 {
		errorsFound = append(errorsFound, fmt.Sprintf(
			"%s.%s=%s currently supports only %s.%s=%s, but got %s.",
			ConfigChecker, ConfigSMTSolver, SMTSolverCVC5, ConfigChecker, ConfigSMTEncoding, SMTEncodingOOPSLA19, encoding))
	}
	if maxError > 1 && configured.View == nil {
		errorsFound = append(errorsFound, fmt.Sprintf("Option %s.%s=%d requires %s.%s.",
			ConfigChecker, ConfigMaxError, maxError, ConfigChecker, ConfigView))
	}
	specification := resolveSpecificationOptions(configured)
	warnings = append(warnings, specification.Warnings()...)
	if !specification.IsSuccess() {
		errorsFound = append(errorsFound, specification.Errors()...)
	}
	if len(errorsFound) != 0 {
		return ConfigFailure[ValidatedCheckOptions](errorsFound, warnings...)
	}
	spec := specification.RequireValue()
	checkDeadlocks := resolveDeadlockSetting(configured, spec, &warnings)
	base := typechecked.RequireValue()
	return ConfigSuccess(ValidatedCheckOptions{
		Common: base.Common, Source: base.Source, Output: base.Output, Typechecker: base.Typechecker,
		Checker: CheckerOptions{
			Algorithm:       requireConfigDefault(withDefaults.Algorithm, ConfigChecker+"."+ConfigAlgorithm),
			DiscardDisabled: requireConfigDefault(withDefaults.DiscardDisabled, ConfigChecker+"."+ConfigDiscardDisabled),
			Length:          requireConfigDefault(withDefaults.Length, ConfigChecker+"."+ConfigLength), MaxError: maxError,
			TimeoutSMTSeconds: requireConfigDefault(withDefaults.TimeoutSMTSeconds, ConfigChecker+"."+ConfigTimeoutSMT),
			CheckDeadlocks:    checkDeadlocks, SMTSolver: solver, SMTEncoding: encoding,
			Tuning: cloneMap(requireConfigDefault(withDefaults.Tuning, ConfigChecker+"."+ConfigTuning)),
		},
		Specification: spec,
	}, warnings...)
}

func ResolveTraceOptions(config ApalacheConfig) ConfigParseResult[ValidatedTraceOptions] {
	checked := ResolveCheckOptions(config)
	if !checked.IsSuccess() {
		return ConfigFailureFrom[ValidatedTraceOptions](checked)
	}
	errorsFound := make([]string, 0, 2)
	trace := config.TraceEvaluation.Trace
	if trace == nil {
		errorsFound = append(errorsFound, "Missing value for required option "+ConfigTracee+"."+ConfigTrace)
	} else if trace.Format() != InputFormatITF && trace.Format() != InputFormatJSON {
		errorsFound = append(errorsFound, "Trace evaluation requires an ITF or JSON trace.")
	}
	if config.TraceEvaluation.Expressions == nil || len(*config.TraceEvaluation.Expressions) == 0 {
		errorsFound = append(errorsFound, "Trace evaluation requires a nonempty list of "+ConfigExpressions+".")
	}
	if len(errorsFound) != 0 {
		return ConfigFailure[ValidatedTraceOptions](errorsFound, checked.Warnings()...)
	}
	base := checked.RequireValue()
	return ConfigSuccess(ValidatedTraceOptions{
		Common: base.Common, Source: base.Source, Output: base.Output, Typechecker: base.Typechecker,
		Checker: base.Checker, Specification: base.Specification,
		TraceEvaluation: TraceEvaluationOptions{Trace: trace, Expressions: append([]string(nil), (*config.TraceEvaluation.Expressions)...)},
	}, checked.Warnings()...)
}

func ResolveServerOptions(config ApalacheConfig) ConfigParseResult[ValidatedServerOptions] {
	command := requireConfigCommand(config)
	if !command.IsSuccess() {
		return ConfigFailureFrom[ValidatedServerOptions](command)
	}
	server := config.MergeWithDefaults().Server
	return ConfigSuccess(ValidatedServerOptions{
		Common: resolveCommonOptions(config),
		Server: ServerOptions{Port: requireConfigDefault(server.Port, ConfigServer+"."+ConfigPort),
			ServerType: requireConfigDefault(server.ServerType, ConfigServer+"."+ConfigServerType)},
	}, command.Warnings()...)
}

func ResolveRemoteCheckOptions(base ApalacheConfig, source InputSource, init, next string, invariants, persistent []string) ConfigParseResult[ValidatedCheckOptions] {
	emptyTuning := map[string]string{}
	command, algorithm := ConfigServer, AlgorithmRemote
	discardDisabled, checkDeadlocks := false, false
	maxError, timeout := 1, 0
	encoding := SMTEncodingOOPSLA19
	request := ApalacheConfig{
		Context: RunContextPatch{Command: &command}, Source: source,
		Checker: CheckerPatch{
			Algorithm: &algorithm, DiscardDisabled: &discardDisabled, MaxError: &maxError,
			TimeoutSMTSeconds: &timeout, CheckDeadlocks: &checkDeadlocks, SMTEncoding: &encoding,
			Tuning: &emptyTuning,
		},
	}
	resolved := ResolveCheckOptions(request.MergeWithLower(base))
	if !resolved.IsSuccess() {
		return ConfigFailureFrom[ValidatedCheckOptions](resolved)
	}
	options := resolved.RequireValue().Clone()
	options.Specification.BehaviorSpec = InitNextSpec{Init: init, Next: next}
	options.Specification.Invariants = append([]string(nil), invariants...)
	options.Specification.Persistent = append([]string(nil), persistent...)
	return ConfigSuccess(options, resolved.Warnings()...)
}

func requireConfigCommand(config ApalacheConfig) ConfigParseResult[string] {
	if config.Context.Command == nil {
		return ConfigFailureMessage[string]("Missing value for required option " + ConfigCommand)
	}
	return ConfigSuccess(*config.Context.Command)
}

func resolveCommonOptions(config ApalacheConfig) CommonOptions {
	common := config.MergeWithDefaults().Common
	return CommonOptions{
		Debug: requireConfigDefault(common.Debug, ConfigDebug), Features: append([]Feature{}, requireConfigDefault(common.Features, ConfigFeatures)...),
		OutDir: requireConfigDefault(common.OutDir, ConfigOutDir), Profiling: requireConfigDefault(common.Profiling, ConfigProfiling),
		RunDir: cloneStringPointer(common.RunDir), SMTProfiling: requireConfigDefault(common.SMTProfiling, ConfigSMTProf),
		WriteIntermediate: requireConfigDefault(common.WriteIntermediate, ConfigWriteIntermediate),
	}
}

func requireConfigDefault[T any](value *T, field string) T {
	if value == nil {
		panic("Missing built-in default for " + field)
	}
	return *value
}

func resolveSpecificationOptions(checker CheckerPatch) ConfigParseResult[SpecificationOptions] {
	if checker.TLCConfig == nil {
		init, next := DefaultInitOperator, DefaultNextOperator
		if checker.Init != nil {
			init = *checker.Init
		}
		if checker.Next != nil {
			next = *checker.Next
		}
		return ConfigSuccess(SpecificationOptions{
			BehaviorSpec: InitNextSpec{Init: init, Next: next}, ConstantInitializer: cloneStringPointer(checker.ConstantInitializer),
			Invariants: cloneOptionalStrings(checker.Invariants), TemporalProperties: cloneOptionalStrings(checker.TemporalProperties),
			View: cloneStringPointer(checker.View), Persistent: []string{},
		})
	}
	loaded := loadResolverTLCConfig(*checker.TLCConfig)
	if !loaded.IsSuccess() {
		return ConfigFailureFrom[SpecificationOptions](loaded)
	}
	tlc := loaded.RequireValue()
	warnings := make([]string, 0)
	behavior := tlc.Behavior
	if fromTLC, ok := behavior.(InitNextSpec); ok {
		behavior = InitNextSpec{
			Init: overrideConfigString(checker.Init, fromTLC.Init, ConfigChecker+"."+ConfigInit, &warnings),
			Next: overrideConfigString(checker.Next, fromTLC.Next, ConfigChecker+"."+ConfigNext, &warnings),
		}
	}
	invariants := overrideConfigList(checker.Invariants, tlc.Invariants, ConfigChecker+"."+ConfigInvariant, &warnings)
	temporal := overrideConfigList(checker.TemporalProperties, tlc.TemporalProperties, ConfigChecker+"."+ConfigTemporal, &warnings)
	input := TLCConfigInput{Config: cloneTLCConfig(tlc), Path: *checker.TLCConfig}
	return ConfigSuccess(SpecificationOptions{
		BehaviorSpec: behavior, ConstantInitializer: cloneStringPointer(checker.ConstantInitializer),
		Invariants: invariants, TemporalProperties: temporal, TLCConfig: &input,
		View: cloneStringPointer(checker.View), Persistent: []string{},
	}, warnings...)
}

func loadResolverTLCConfig(path string) ConfigParseResult[TLCConfig] {
	if !pathExists(path) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			absolute = path
		}
		return ConfigFailureMessage[TLCConfig]("Specified TLC config file not found: " + absolute)
	}
	reader, err := os.Open(path)
	if err != nil {
		return ConfigFailureMessage[TLCConfig](fmt.Sprintf("%s: IO error when loading the TLC config: %v", filepath.Base(path), err))
	}
	defer reader.Close()
	config, err := ParseTLCConfigReader(reader)
	if err == nil {
		return ConfigSuccess(config)
	}
	var parseError *TLCConfigParseError
	if errors.As(err, &parseError) {
		return ConfigFailureMessage[TLCConfig](fmt.Sprintf("%s:line %d, column %d: Error parsing the TLC config file: %s",
			filepath.Base(path), parseError.Line, parseError.Column, parseError.Message))
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || err != nil {
		return ConfigFailureMessage[TLCConfig](fmt.Sprintf("%s: IO error when loading the TLC config: %v", filepath.Base(path), err))
	}
	panic("unreachable")
}

func resolveDeadlockSetting(checker CheckerPatch, specification SpecificationOptions, warnings *[]string) bool {
	var fromTLC *bool
	if specification.TLCConfig != nil {
		fromTLC = specification.TLCConfig.Config.CheckDeadlock
	}
	if fromTLC != nil && checker.CheckDeadlocks != nil {
		if *fromTLC != *checker.CheckDeadlocks {
			*warnings = append(*warnings, fmt.Sprintf("TLC CHECK_DEADLOCK=%t is overridden by %s.%s=%t.",
				*fromTLC, ConfigChecker, ConfigNoDeadlock, !*checker.CheckDeadlocks))
		}
		return *checker.CheckDeadlocks
	}
	if fromTLC != nil {
		*warnings = append(*warnings, fmt.Sprintf("Using CHECK_DEADLOCK=%t from the TLC configuration.", *fromTLC))
		return *fromTLC
	}
	if checker.CheckDeadlocks != nil {
		return *checker.CheckDeadlocks
	}
	return DefaultCheckDeadlocks
}

func overrideConfigString(configured *string, fromTLC, field string, warnings *[]string) string {
	if configured != nil {
		if fromTLC != "" {
			*warnings = append(*warnings, field+" overrides the value from the TLC configuration.")
		}
		return *configured
	}
	return fromTLC
}

func overrideConfigList(configured *[]string, fromTLC []string, field string, warnings *[]string) []string {
	if configured != nil {
		if len(fromTLC) != 0 {
			*warnings = append(*warnings, field+" overrides the value from the TLC configuration.")
		}
		return append([]string(nil), (*configured)...)
	}
	return append([]string(nil), fromTLC...)
}

func cloneOptionalStrings(values *[]string) []string {
	if values == nil {
		return []string{}
	}
	return append([]string(nil), (*values)...)
}
