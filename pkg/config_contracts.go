package apalache

import (
	"fmt"
	"strings"
)

const (
	ConfigCheck             = "check"
	ConfigConfig            = "config"
	ConfigParse             = "parse"
	ConfigServer            = "server"
	ConfigSimulate          = "simulate"
	ConfigTest              = "test"
	ConfigTracee            = "tracee"
	ConfigTranspile         = "transpile"
	ConfigTypecheck         = "typecheck"
	ConfigChecker           = "checker"
	ConfigCommand           = "command"
	ConfigFile              = "config-file"
	ConfigDebug             = "debug"
	ConfigFeatures          = "features"
	ConfigOutput            = "output"
	ConfigOutDir            = "out-dir"
	ConfigProfiling         = "profiling"
	ConfigRunDir            = "run-dir"
	ConfigSMTProf           = "smtprof"
	ConfigSource            = "source"
	ConfigTypechecker       = "typechecker"
	ConfigWriteIntermediate = "write-intermediate"
	ConfigAlgorithm         = "algo"
	ConfigCInit             = "cinit"
	ConfigDiscardDisabled   = "discard-disabled"
	ConfigInit              = "init"
	ConfigInvariant         = "inv"
	ConfigLength            = "length"
	ConfigMaxError          = "max-error"
	ConfigNext              = "next"
	ConfigNoDeadlock        = "no-deadlock"
	ConfigSMTEncoding       = "smt-encoding"
	ConfigSMTSolver         = "smt-solver"
	ConfigTemporal          = "temporal"
	ConfigTimeoutSMT        = "timeout-smt"
	ConfigTuning            = "tuning"
	ConfigView              = "view"
	ConfigAux               = "aux"
	ConfigContent           = "content"
	ConfigExpressions       = "expressions"
	ConfigFileSource        = "file"
	ConfigFormat            = "format"
	ConfigInferPoly         = "infer-poly"
	ConfigKind              = "kind"
	ConfigPath              = "path"
	ConfigPort              = "port"
	ConfigServerType        = "server-type"
	ConfigTrace             = "trace"
	ConfigStringSource      = "string"
	ConfigAction            = "action"
	ConfigAssertion         = "assertion"
	ConfigBefore            = "before"
	ConfigEnableStats       = "enable-stats"
	ConfigMaxRun            = "max-run"
	ConfigOutputTraces      = "output-traces"
	ConfigTuningOptions     = "tuning-options"
	ConfigTuningOptionsFile = "tuning-options-file"
	LegacyConfigExtension   = ".cfg"
	GlobalConfigFilename    = "apalache.json"
	JSONConfigExtension     = ".json"
	LocalConfigFilename     = ".apalache.json"
	TLAPlusConfigDirectory  = ".tlaplus"
	ConfigUserHomeProperty  = "user.home"
)

func SMTEncodingValues() []SMTEncoding {
	return []SMTEncoding{SMTEncodingOOPSLA19, SMTEncodingArrays, SMTEncodingFunArrays}
}

func SMTSolverValues() []SMTSolverBackend { return []SMTSolverBackend{SMTSolverZ3, SMTSolverCVC5} }

func ParseSMTSolver(value string) (SMTSolverBackend, error) { return ParseSMTSolverBackend(value) }

type CheckerAlgorithm string

const (
	AlgorithmIncremental CheckerAlgorithm = "incremental"
	AlgorithmOffline     CheckerAlgorithm = "offline"
	AlgorithmRemote      CheckerAlgorithm = "remote"
)

func CheckerAlgorithmValues() []CheckerAlgorithm {
	return []CheckerAlgorithm{AlgorithmIncremental, AlgorithmOffline, AlgorithmRemote}
}

func ParseCheckerAlgorithm(value string) (CheckerAlgorithm, error) {
	normalized := strings.ToLower(value)
	switch CheckerAlgorithm(normalized) {
	case AlgorithmIncremental, AlgorithmOffline, AlgorithmRemote:
		return CheckerAlgorithm(normalized), nil
	default:
		return "", fmt.Errorf("Unexpected checker algorithm: %s", normalized)
	}
}

type ServerType string

const (
	ServerTypeChecker  ServerType = "checker"
	ServerTypeExplorer ServerType = "explorer"
)

func ServerTypeValues() []ServerType { return []ServerType{ServerTypeChecker, ServerTypeExplorer} }

func ParseServerType(value string) (ServerType, error) {
	normalized := strings.ToLower(value)
	switch ServerType(normalized) {
	case ServerTypeChecker, ServerTypeExplorer:
		return ServerType(normalized), nil
	default:
		return "", fmt.Errorf("Unexpected server type: %s", normalized)
	}
}
