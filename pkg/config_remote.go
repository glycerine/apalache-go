package apalache

import "sort"

var remoteFileWritingTuningKeys = map[string]struct{}{
	"z3.dot_proof_file":            {},
	"z3.trace":                     {},
	"z3.trace_file_name":           {},
	"z3.sat.drat.file":             {},
	"z3.sat.inprocess.out":         {},
	"z3.solver.axioms2files":       {},
	"z3.solver.cancel_backup_file": {},
	"z3.solver.proof.log":          {},
	"z3.solver.smtlib2_log":        {},
	"z3.opt.dump_benchmarks":       {},
	"z3.opt.solution_prefix":       {},
	"z3.smt.arith.dump_lemmas":     {},
}

// RemoteFileWritingTuningKeys returns the solver tuning options that may
// create caller-selected or implicit files.
func RemoteFileWritingTuningKeys() []string {
	keys := make([]string, 0, len(remoteFileWritingTuningKeys))
	for key := range remoteFileWritingTuningKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ParseRemoteApalacheConfig parses an untrusted service configuration without
// loading any configuration file and then applies the remote safety policy.
func ParseRemoteApalacheConfig(sourceText string, sourceName ...string) ConfigParseResult[ApalacheConfig] {
	name := "<RPC configuration>"
	if len(sourceName) > 0 && sourceName[0] != "" {
		name = sourceName[0]
	}
	parsed := ParseApalacheConfigJSON(sourceText, name)
	if !parsed.IsSuccess() {
		return ConfigFailureFrom[ApalacheConfig](parsed)
	}
	return ConfigWithWarnings(ValidateRemoteApalacheConfig(parsed.RequireValue()), parsed.Warnings()...)
}

// ValidateRemoteApalacheConfig rejects request-controlled paths and solver
// settings capable of writing files.
func ValidateRemoteApalacheConfig(config ApalacheConfig) ConfigParseResult[ApalacheConfig] {
	errors := make([]string, 0)
	rejectRemotePath(config.Context.ConfigFile != nil, "$.config-file", &errors)
	rejectRemotePath(config.Common.OutDir != nil, "$.out-dir", &errors)
	rejectRemotePath(config.Common.RunDir != nil, "$.run-dir", &errors)
	rejectRemotePath(config.Output != nil, "$.output", &errors)
	rejectRemotePath(config.Checker.TLCConfig != nil, "$.checker.config", &errors)
	if isFileInputSource(config.Source) {
		errors = append(errors, "$.source: File-backed sources are not allowed in remote configuration; provide in-memory content.")
	}
	if isFileInputSource(config.TraceEvaluation.Trace) {
		errors = append(errors, "$.tracee.trace: File-backed sources are not allowed in remote configuration; provide in-memory content.")
	}
	if config.Checker.Tuning != nil {
		unsafe := make([]string, 0)
		for key := range *config.Checker.Tuning {
			if _, found := remoteFileWritingTuningKeys[key]; found {
				unsafe = append(unsafe, key)
			}
		}
		sort.Strings(unsafe)
		for _, key := range unsafe {
			errors = append(errors, "$.checker.tuning."+key+": This tuning option can write files and is not allowed in remote configuration.")
		}
	}
	if len(errors) != 0 {
		return ConfigFailure[ApalacheConfig](errors)
	}
	return ConfigSuccess(config)
}

func rejectRemotePath(present bool, path string, errors *[]string) {
	if present {
		*errors = append(*errors, path+": Filesystem paths are not allowed in remote configuration.")
	}
}

func isFileInputSource(source InputSource) bool {
	switch source.(type) {
	case FileInputSource, *FileInputSource:
		return true
	default:
		return false
	}
}
