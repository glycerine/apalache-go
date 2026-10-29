package apalache

import (
	"reflect"
	"testing"
)

func TestConfigurationEnumsMatchScalaValuesOrderAndParsing(t *testing.T) {
	if got, want := SMTEncodingValues(), []SMTEncoding{"oopsla19", "arrays", "funArrays"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SMT encodings = %#v", got)
	}
	if got, want := SMTSolverValues(), []SMTSolverBackend{"z3", "cvc5"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SMT solvers = %#v", got)
	}
	if got, want := CheckerAlgorithmValues(), []CheckerAlgorithm{"incremental", "offline", "remote"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("algorithms = %#v", got)
	}
	if got, want := ServerTypeValues(), []ServerType{"checker", "explorer"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server types = %#v", got)
	}
	if encoding, err := ParseSMTEncoding("funArrays"); err != nil || encoding != SMTEncodingFunArrays {
		t.Fatalf("encoding = %q, %v", encoding, err)
	}
	if _, err := ParseSMTEncoding("FUNARRAYS"); err == nil || err.Error() != "Unexpected SMT encoding: FUNARRAYS" {
		t.Fatalf("case-sensitive encoding error = %v", err)
	}
	for _, deprecated := range []string{"fun-arrays", "oopsla-19"} {
		if _, err := ParseSMTEncoding(deprecated); err == nil {
			t.Fatalf("deprecated SMT encoding alias %q was accepted", deprecated)
		}
	}
	if solver, err := ParseSMTSolver("Z3"); err != nil || solver != SMTSolverZ3 {
		t.Fatalf("solver = %q, %v", solver, err)
	}
	if algorithm, err := ParseCheckerAlgorithm("REMOTE"); err != nil || algorithm != AlgorithmRemote {
		t.Fatalf("algorithm = %q, %v", algorithm, err)
	}
	if server, err := ParseServerType("Explorer"); err != nil || server != ServerTypeExplorer {
		t.Fatalf("server type = %q, %v", server, err)
	}
	if _, err := ParseSMTSolver("Boolector"); err == nil || err.Error() != "Unexpected SMT solver backend: boolector" {
		t.Fatalf("solver error = %v", err)
	}
	if _, err := ParseCheckerAlgorithm("Parallel"); err == nil || err.Error() != "Unexpected checker algorithm: parallel" {
		t.Fatalf("algorithm error = %v", err)
	}
	if _, err := ParseServerType("HTTP"); err == nil || err.Error() != "Unexpected server type: http" {
		t.Fatalf("server error = %v", err)
	}

	encodings := SMTEncodingValues()
	encodings[0] = "changed"
	if SMTEncodingValues()[0] != SMTEncodingOOPSLA19 {
		t.Fatal("SMT encoding values exposed shared storage")
	}
}

func TestConfigurationConstantsMatchSharedVocabulary(t *testing.T) {
	got := []string{
		ConfigCheck, ConfigConfig, ConfigParse, ConfigServer, ConfigSimulate, ConfigTest, ConfigTracee, ConfigTranspile,
		ConfigTypecheck, ConfigChecker, ConfigCommand, ConfigFile, ConfigDebug, ConfigFeatures, ConfigOutput, ConfigOutDir,
		ConfigProfiling, ConfigRunDir, ConfigSMTProf, ConfigSource, ConfigTypechecker, ConfigWriteIntermediate,
		ConfigAlgorithm, ConfigCInit, ConfigDiscardDisabled, ConfigInit, ConfigInvariant, ConfigLength, ConfigMaxError,
		ConfigNext, ConfigNoDeadlock, ConfigSMTEncoding, ConfigSMTSolver, ConfigTemporal, ConfigTimeoutSMT, ConfigTuning,
		ConfigView, ConfigAux, ConfigContent, ConfigExpressions, ConfigFileSource, ConfigFormat, ConfigInferPoly, ConfigKind,
		ConfigPath, ConfigPort, ConfigServerType, ConfigTrace, ConfigStringSource, ConfigAction, ConfigAssertion, ConfigBefore,
		ConfigEnableStats, ConfigMaxRun, ConfigOutputTraces, ConfigTuningOptions, ConfigTuningOptionsFile,
		LegacyConfigExtension, GlobalConfigFilename, JSONConfigExtension, LocalConfigFilename, TLAPlusConfigDirectory,
		ConfigUserHomeProperty,
	}
	want := []string{
		"check", "config", "parse", "server", "simulate", "test", "tracee", "transpile", "typecheck", "checker",
		"command", "config-file", "debug", "features", "output", "out-dir", "profiling", "run-dir", "smtprof",
		"source", "typechecker", "write-intermediate", "algo", "cinit", "discard-disabled", "init", "inv", "length",
		"max-error", "next", "no-deadlock", "smt-encoding", "smt-solver", "temporal", "timeout-smt", "tuning", "view",
		"aux", "content", "expressions", "file", "format", "infer-poly", "kind", "path", "port", "server-type", "trace",
		"string", "action", "assertion", "before", "enable-stats", "max-run", "output-traces", "tuning-options",
		"tuning-options-file", ".cfg", "apalache.json", ".json", ".apalache.json", ".tlaplus", "user.home",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuration vocabulary = %#v", got)
	}
}
