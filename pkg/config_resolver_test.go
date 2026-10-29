package apalache

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveCommandParseAndTypecheckOptions(t *testing.T) {
	missing := ResolveCommandInitialization(EmptyApalacheConfig())
	if missing.IsSuccess() || !reflect.DeepEqual(missing.Errors(), []string{"Missing value for required option command"}) {
		t.Fatalf("missing command = %#v", missing.Errors())
	}
	config := EmptyApalacheConfig().WithCommand(ConfigParse)
	parsed := ResolveParseOptions(config)
	if parsed.IsSuccess() || !reflect.DeepEqual(parsed.Errors(), []string{"Missing value for required option source"}) {
		t.Fatalf("missing source = %#v", parsed.Errors())
	}
	source := NewStringInputSource("---- MODULE M ----\n====", nil, InputFormatTLA)
	output, infer := "out.tla", false
	config.Source, config.Output, config.Typechecker.InferPoly = source, &output, &infer
	initialized := ResolveCommandInitialization(config)
	if !initialized.IsSuccess() || initialized.RequireValue().Command != ConfigParse ||
		!reflect.DeepEqual(initialized.RequireValue().Source, source) {
		t.Fatalf("initialization = %#v, %#v", initialized.Errors(), initialized.RequireValue())
	}
	typechecked := ResolveTypecheckOptions(config)
	if !typechecked.IsSuccess() || typechecked.RequireValue().Typechecker.InferPoly ||
		typechecked.RequireValue().Output == nil || *typechecked.RequireValue().Output != output {
		t.Fatalf("typecheck = %#v", typechecked.Errors())
	}
}

func TestResolveCheckAndServerUseStaticDefaults(t *testing.T) {
	config := resolverCheckConfig()
	result := ResolveCheckOptions(config)
	if !result.IsSuccess() {
		t.Fatal(result.Errors())
	}
	options := result.RequireValue()
	defaults := DefaultApalacheConfig()
	if options.Common.Debug != *defaults.Common.Debug || !reflect.DeepEqual(options.Common.Features, *defaults.Common.Features) ||
		options.Common.OutDir != *defaults.Common.OutDir || options.Common.Profiling != *defaults.Common.Profiling ||
		options.Common.SMTProfiling != *defaults.Common.SMTProfiling ||
		options.Common.WriteIntermediate != *defaults.Common.WriteIntermediate ||
		options.Typechecker.InferPoly != *defaults.Typechecker.InferPoly ||
		options.Checker.Algorithm != *defaults.Checker.Algorithm ||
		options.Checker.DiscardDisabled != *defaults.Checker.DiscardDisabled ||
		options.Checker.Length != *defaults.Checker.Length || options.Checker.MaxError != *defaults.Checker.MaxError ||
		options.Checker.TimeoutSMTSeconds != *defaults.Checker.TimeoutSMTSeconds ||
		options.Checker.CheckDeadlocks != DefaultCheckDeadlocks || options.Checker.SMTSolver != *defaults.Checker.SMTSolver ||
		options.Checker.SMTEncoding != *defaults.Checker.SMTEncoding || !reflect.DeepEqual(options.Checker.Tuning, *defaults.Checker.Tuning) {
		t.Fatalf("resolved defaults = %#v", options)
	}
	server := ResolveServerOptions(config.WithCommand(ConfigServer))
	if !server.IsSuccess() || server.RequireValue().Server.Port != *defaults.Server.Port ||
		server.RequireValue().Server.ServerType != *defaults.Server.ServerType {
		t.Fatalf("server = %#v, %#v", server.Errors(), server.RequireValue())
	}
}

func TestResolveCheckSolverEncodingCompatibilityMatchesScala(t *testing.T) {
	encodings := []SMTEncoding{SMTEncodingOOPSLA19, SMTEncodingArrays, SMTEncodingFunArrays}
	for _, encoding := range encodings {
		config := resolverCheckConfig()
		config.Checker.SMTSolver = pointer(SMTSolverZ3)
		config.Checker.SMTEncoding = pointer(encoding)
		if result := ResolveCheckOptions(config); !result.IsSuccess() {
			t.Errorf("z3/%s = %#v", encoding, result.Errors())
		}
	}
	for _, encoding := range encodings {
		config := resolverCheckConfig()
		config.Checker.SMTSolver = pointer(SMTSolverCVC5)
		config.Checker.SMTEncoding = pointer(encoding)
		result := ResolveCheckOptions(config)
		if encoding == SMTEncodingOOPSLA19 {
			if !result.IsSuccess() {
				t.Errorf("cvc5/%s = %#v", encoding, result.Errors())
			}
			continue
		}
		joined := strings.Join(result.Errors(), "\n")
		for _, fragment := range []string{"checker.smt-solver=cvc5", "checker.smt-encoding=oopsla19", string(encoding)} {
			if result.IsSuccess() || !strings.Contains(joined, fragment) {
				t.Errorf("cvc5/%s errors %q do not contain %q", encoding, joined, fragment)
			}
		}
	}
}

func TestResolveCheckCrossFieldValidationAccumulates(t *testing.T) {
	config := resolverCheckConfig()
	config.Checker.SMTSolver = pointer(SMTSolverCVC5)
	config.Checker.SMTEncoding = pointer(SMTEncodingArrays)
	config.Checker.MaxError = pointer(2)
	missing := filepath.Join(t.TempDir(), "missing.cfg")
	config.Checker.TLCConfig = &missing
	result := ResolveCheckOptions(config)
	joined := strings.Join(result.Errors(), "\n")
	for _, fragment := range []string{
		"checker.smt-solver=cvc5", "checker.smt-encoding=oopsla19", "arrays",
		"Option checker.max-error=2 requires checker.view", "Specified TLC config file not found",
	} {
		if !strings.Contains(joined, fragment) {
			t.Errorf("errors %q do not contain %q", joined, fragment)
		}
	}
	if result.IsSuccess() || len(result.Errors()) != 3 {
		t.Fatalf("cross-field errors = %#v", result.Errors())
	}
	config.Checker.View = pointer("View")
	config.Checker.TLCConfig = nil
	config.Checker.SMTEncoding = pointer(SMTEncodingOOPSLA19)
	if valid := ResolveCheckOptions(config); !valid.IsSuccess() {
		t.Fatalf("valid CVC5 configuration = %#v", valid.Errors())
	}
}

func TestResolveSpecificationDefaultsAndConfiguredLists(t *testing.T) {
	config := resolverCheckConfig()
	config.Checker.Init, config.Checker.Next = pointer("Start"), pointer("Step")
	empty := []string{}
	temporal := []string{"Live"}
	config.Checker.Invariants, config.Checker.TemporalProperties = &empty, &temporal
	result := ResolveCheckOptions(config)
	if !result.IsSuccess() {
		t.Fatal(result.Errors())
	}
	spec := result.RequireValue().Specification
	if spec.BehaviorSpec != (InitNextSpec{Init: "Start", Next: "Step"}) || len(spec.Invariants) != 0 ||
		!reflect.DeepEqual(spec.TemporalProperties, temporal) || spec.TLCConfig != nil {
		t.Fatalf("specification = %#v", spec)
	}
	defaults := ResolveCheckOptions(resolverCheckConfig()).RequireValue().Specification.BehaviorSpec
	if defaults != (InitNextSpec{Init: DefaultInitOperator, Next: DefaultNextOperator}) {
		t.Fatalf("default behavior = %#v", defaults)
	}
}

func TestResolveTLCConfigurationOverridesAndDeadlockPrecedence(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "M.cfg")
	writeConfigTestFile(t, path, "INIT TlcInit\nNEXT TlcNext\nINVARIANTS TlcInv\nPROPERTIES TlcLive\nCHECK_DEADLOCK FALSE\n")
	config := resolverCheckConfig()
	config.Checker.TLCConfig = &path
	fromTLC := ResolveCheckOptions(config)
	if !fromTLC.IsSuccess() {
		t.Fatal(fromTLC.Errors())
	}
	options := fromTLC.RequireValue()
	if options.Specification.BehaviorSpec != (InitNextSpec{Init: "TlcInit", Next: "TlcNext"}) ||
		!reflect.DeepEqual(options.Specification.Invariants, []string{"TlcInv"}) ||
		!reflect.DeepEqual(options.Specification.TemporalProperties, []string{"TlcLive"}) || options.Checker.CheckDeadlocks ||
		len(fromTLC.Warnings()) != 1 || !strings.Contains(fromTLC.Warnings()[0], "Using CHECK_DEADLOCK=false") {
		t.Fatalf("TLC options = %#v, warnings %#v", options, fromTLC.Warnings())
	}
	config.Checker.Init, config.Checker.Next = pointer("AppInit"), pointer("AppNext")
	config.Checker.Invariants = &[]string{}
	config.Checker.TemporalProperties = &[]string{"AppLive"}
	config.Checker.CheckDeadlocks = pointer(true)
	overridden := ResolveCheckOptions(config)
	if !overridden.IsSuccess() || !overridden.RequireValue().Checker.CheckDeadlocks ||
		overridden.RequireValue().Specification.BehaviorSpec != (InitNextSpec{Init: "AppInit", Next: "AppNext"}) ||
		len(overridden.RequireValue().Specification.Invariants) != 0 ||
		!reflect.DeepEqual(overridden.RequireValue().Specification.TemporalProperties, []string{"AppLive"}) || len(overridden.Warnings()) != 5 {
		t.Fatalf("overridden TLC = %#v, %#v", overridden.Errors(), overridden.Warnings())
	}
}

func TestResolveTLCConfigurationTemporalAndFailures(t *testing.T) {
	directory := t.TempDir()
	temporalPath := filepath.Join(directory, "Temporal.cfg")
	writeConfigTestFile(t, temporalPath, "SPECIFICATION Spec\n")
	config := resolverCheckConfig()
	config.Checker.TLCConfig = &temporalPath
	resolved := ResolveCheckOptions(config)
	if !resolved.IsSuccess() || resolved.RequireValue().Specification.BehaviorSpec != (TemporalSpec{Name: "Spec"}) {
		t.Fatalf("temporal config = %#v, %#v", resolved.Errors(), resolved.RequireValue().Specification)
	}
	badPath := filepath.Join(directory, "Bad.cfg")
	writeConfigTestFile(t, badPath, "INIT")
	config.Checker.TLCConfig = &badPath
	bad := ResolveCheckOptions(config)
	if bad.IsSuccess() || !strings.Contains(strings.Join(bad.Errors(), " "), "Bad.cfg:line") ||
		!strings.Contains(strings.Join(bad.Errors(), " "), "Error parsing the TLC config file") {
		t.Fatalf("parse failure = %#v", bad.Errors())
	}
	config.Checker.TLCConfig = &directory
	read := ResolveCheckOptions(config)
	if read.IsSuccess() || !strings.Contains(strings.Join(read.Errors(), " "), "IO error when loading the TLC config") {
		t.Fatalf("read failure = %#v", read.Errors())
	}
}

func TestResolveTraceOptionsValidationAndSuccess(t *testing.T) {
	config := resolverCheckConfig().WithCommand(ConfigTracee)
	missing := ResolveTraceOptions(config)
	if missing.IsSuccess() || len(missing.Errors()) != 2 {
		t.Fatalf("missing trace = %#v", missing.Errors())
	}
	expressions := []string{"X"}
	config.TraceEvaluation.Expressions = &expressions
	config.TraceEvaluation.Trace = NewStringInputSource("{}", nil, InputFormatTLA)
	wrong := ResolveTraceOptions(config)
	if wrong.IsSuccess() || !reflect.DeepEqual(wrong.Errors(), []string{"Trace evaluation requires an ITF or JSON trace."}) {
		t.Fatalf("wrong trace = %#v", wrong.Errors())
	}
	config.TraceEvaluation.Trace = NewStringInputSource("{}", nil, InputFormatJSON)
	valid := ResolveTraceOptions(config)
	if !valid.IsSuccess() || !reflect.DeepEqual(valid.RequireValue().TraceEvaluation.Expressions, expressions) {
		t.Fatalf("valid trace = %#v", valid.Errors())
	}
	expressions[0] = "mutated"
	if valid.RequireValue().TraceEvaluation.Expressions[0] != "X" {
		t.Fatal("trace expressions alias the sparse configuration")
	}
}

func TestResolveRemoteCheckOptionsForcesRequestPolicyAndPredicates(t *testing.T) {
	base := resolverCheckConfig()
	base.Checker.Algorithm = pointer(AlgorithmOffline)
	base.Checker.DiscardDisabled = pointer(true)
	base.Checker.MaxError = pointer(5)
	base.Checker.CheckDeadlocks = pointer(true)
	base.Checker.SMTEncoding = pointer(SMTEncodingArrays)
	base.Checker.Tuning = &map[string]string{"search.invariant.mode": "after"}
	source := NewStringInputSource("---- MODULE M ----\n====", nil, InputFormatTLA)
	invariants, persistent := []string{"Inv"}, []string{"Persist"}
	result := ResolveRemoteCheckOptions(base, source, "RemoteInit", "RemoteNext", invariants, persistent)
	if !result.IsSuccess() {
		t.Fatal(result.Errors())
	}
	options := result.RequireValue()
	if options.Checker.Algorithm != AlgorithmRemote || options.Checker.DiscardDisabled || options.Checker.MaxError != 1 ||
		options.Checker.CheckDeadlocks || options.Checker.SMTEncoding != SMTEncodingOOPSLA19 ||
		!reflect.DeepEqual(options.Checker.Tuning, map[string]string{"search.invariant.mode": "after"}) ||
		options.Specification.BehaviorSpec != (InitNextSpec{Init: "RemoteInit", Next: "RemoteNext"}) ||
		!reflect.DeepEqual(options.Specification.Invariants, invariants) || !reflect.DeepEqual(options.Specification.Persistent, persistent) {
		t.Fatalf("remote options = %#v", options)
	}
}

func resolverCheckConfig() ApalacheConfig {
	return ApalacheConfig{
		Context: RunContextPatch{Command: pointer(ConfigCheck)},
		Source:  NewStringInputSource("---- MODULE M ----\n====", nil, InputFormatTLA),
	}
}

func TestResolveTLCConfigPathIsUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Unicode.cfg")
	if err := os.WriteFile(path, []byte("INIT Α\nNEXT Β\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The current TLC grammar intentionally accepts ASCII identifiers only;
	// this proves UTF-8 input reaches the parser and is reported as a parse
	// error rather than a decoding or I/O failure.
	config := resolverCheckConfig()
	config.Checker.TLCConfig = &path
	result := ResolveCheckOptions(config)
	if result.IsSuccess() || !strings.Contains(strings.Join(result.Errors(), " "), "Error parsing") {
		t.Fatalf("UTF-8 parser error = %#v", result.Errors())
	}
}
