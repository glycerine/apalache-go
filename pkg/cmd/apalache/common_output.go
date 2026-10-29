package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	pal "github.com/glycerine/apalache-go"
)

type commonOutputFlags struct {
	debug             optionalBoolFlag
	writeIntermediate optionalBoolFlag
	profiling         optionalBoolFlag
}

type commonOutputOptions struct {
	debug             bool
	writeIntermediate bool
	profiling         bool
}

func addCommonOutputFlags(flags *flag.FlagSet) *commonOutputFlags {
	options := &commonOutputFlags{}
	flags.Var(&options.debug, "debug", "write the resolved application configuration to the run directory")
	flags.Var(&options.writeIntermediate, "write-intermediate", "write intermediate representations to the run directory")
	flags.Var(&options.profiling, "profiling", "write rewriting-rule statistics to profile-rules.txt")
	return options
}

func (flags *commonOutputFlags) resolve(config jsonCommandConfig, visited map[string]bool) (commonOutputOptions, error) {
	options := commonOutputOptions{}
	if visited["debug"] {
		options.debug = flags.debug.value
	} else if config.Debug != nil {
		options.debug = *config.Debug
	}
	if visited["write-intermediate"] {
		options.writeIntermediate = flags.writeIntermediate.value
	} else if config.WriteIntermediate != nil {
		options.writeIntermediate = *config.WriteIntermediate
	} else if value, found := os.LookupEnv("WRITE_INTERMEDIATE"); found {
		var parsed optionalBoolFlag
		if err := parsed.Set(value); err != nil {
			return commonOutputOptions{}, fmt.Errorf("invalid WRITE_INTERMEDIATE environment value %q: %w", value, err)
		}
		options.writeIntermediate = parsed.value
	}
	if visited["profiling"] {
		options.profiling = flags.profiling.value
	} else if config.Profiling != nil {
		options.profiling = *config.Profiling
	} else if value, found := os.LookupEnv("PROFILING"); found {
		var parsed optionalBoolFlag
		if err := parsed.Set(value); err != nil {
			return commonOutputOptions{}, fmt.Errorf("invalid PROFILING environment value %q: %w", value, err)
		}
		options.profiling = parsed.value
	}
	return options, nil
}

func newRuleProfiler(enabled bool) *pal.RuleStatListener {
	if !enabled {
		return nil
	}
	return pal.NewRuleStatListener()
}

func writeRuleProfile(paths commandOutputPaths, listener *pal.RuleStatListener) error {
	if listener == nil {
		return nil
	}
	var report bytes.Buffer
	if err := listener.Locator().WriteStats(&report); err != nil {
		return fmt.Errorf("format rule profile: %w", err)
	}
	for _, path := range paths.implicitFiles("profile-rules.txt") {
		if err := os.WriteFile(path, report.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write rule profile to %s: %w", path, err)
		}
	}
	return nil
}

func writeDebugApplicationConfig(
	paths commandOutputPaths,
	config jsonCommandConfig,
	command, selectedConfig, sourcePath, outDir, runDir string,
	options commonOutputOptions,
	effectiveOverlays ...map[string]any,
) error {
	if !options.debug {
		return nil
	}
	config.Command = stringPointer(command)
	if selectedConfig != "" {
		config.ConfigFile = stringPointer(selectedConfig)
	}
	config.OutDir = stringPointer(outDir)
	if runDir != "" {
		config.RunDir = stringPointer(runDir)
	}
	config.Debug = boolPointer(true)
	config.WriteIntermediate = boolPointer(options.writeIntermediate)
	if config.SMTProfile == nil {
		config.SMTProfile = boolPointer(false)
	}
	config.Profiling = boolPointer(options.profiling)
	if config.Features == nil {
		empty := []string{}
		config.Features = &empty
	}
	if config.Source == nil && sourcePath != "" {
		config.Source = &jsonInputSource{Kind: stringPointer("file"), Path: stringPointer(sourcePath)}
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode application configuration: %w", err)
	}
	var snapshot map[string]any
	if err = json.Unmarshal(encoded, &snapshot); err != nil {
		return fmt.Errorf("normalize application configuration: %w", err)
	}
	pruneNullConfigurationValues(snapshot)
	for _, overlay := range effectiveOverlays {
		mergeDebugConfiguration(snapshot, overlay)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("format application configuration: %w", err)
	}
	data = append(data, '\n')
	for _, path := range paths.implicitFiles("application-config.json") {
		if err = os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write application configuration to %s: %w", path, err)
		}
	}
	return nil
}

func mergeDebugConfiguration(target, overlay map[string]any) {
	for key, value := range overlay {
		incoming, nested := value.(map[string]any)
		if !nested {
			target[key] = value
			continue
		}
		existing, ok := target[key].(map[string]any)
		if !ok {
			existing = make(map[string]any)
			target[key] = existing
		}
		mergeDebugConfiguration(existing, incoming)
	}
}

func checkDebugApplicationOverlay(
	backend *checkerBackendOptions,
	tuning map[string]string,
	TLCConfig, constantInit, init, next string,
	invariants, temporal []string,
	length, maxErrors, timeoutSMT int,
	discardDisabled, checkDeadlocks, inferPoly, rows bool,
	view string,
) map[string]any {
	checker := map[string]any{
		"tuning":           cloneStringMap(tuning),
		"algo":             backend.algorithm,
		"discard-disabled": discardDisabled,
		"init":             init,
		"inv":              append([]string(nil), invariants...),
		"next":             next,
		"length":           length,
		"max-error":        maxErrors,
		"timeout-smt":      timeoutSMT,
		"no-deadlock":      !checkDeadlocks,
		"smt-solver":       backend.solver,
		"smt-encoding":     backend.encoding,
		"temporal":         append([]string(nil), temporal...),
	}
	if TLCConfig != "" {
		checker["config"] = TLCConfig
	}
	if constantInit != "" {
		checker["cinit"] = constantInit
	}
	if view != "" {
		checker["view"] = view
	}
	features := []string{}
	if rows {
		features = append(features, "rows")
	}
	return map[string]any{
		"features":    features,
		"checker":     checker,
		"typechecker": map[string]any{"infer-poly": inferPoly},
	}
}

func frontendDebugApplicationOverlay(output string, rows, inferPoly bool) map[string]any {
	features := []string{}
	if rows {
		features = append(features, "rows")
	}
	return map[string]any{
		"output":      output,
		"features":    features,
		"typechecker": map[string]any{"infer-poly": inferPoly},
	}
}

func traceeDebugApplicationOverlay(trace string, expressions []string, rows, inferPoly bool) map[string]any {
	overlay := frontendDebugApplicationOverlay("", rows, inferPoly)
	delete(overlay, "output")
	overlay["tracee"] = map[string]any{
		"trace":       trace,
		"expressions": append([]string(nil), expressions...),
	}
	return overlay
}

func transpileDebugApplicationOverlay(
	output, TLCConfig, constantInit, init, next string,
	invariants, temporal []string,
	rows, inferPoly bool,
) map[string]any {
	overlay := frontendDebugApplicationOverlay(output, rows, inferPoly)
	checker := map[string]any{
		"init":     init,
		"inv":      append([]string(nil), invariants...),
		"next":     next,
		"temporal": append([]string(nil), temporal...),
	}
	if TLCConfig != "" {
		checker["config"] = TLCConfig
	}
	if constantInit != "" {
		checker["cinit"] = constantInit
	}
	overlay["checker"] = checker
	return overlay
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func pruneNullConfigurationValues(object map[string]any) {
	for key, value := range object {
		switch nested := value.(type) {
		case nil:
			delete(object, key)
		case map[string]any:
			pruneNullConfigurationValues(nested)
			if len(nested) == 0 {
				delete(object, key)
			}
		}
	}
}

func boolPointer(value bool) *bool { return &value }

func writeModuleIntermediate(
	paths commandOutputPaths,
	module *pal.Module,
	extends []string,
	sources *pal.SourceStore,
	baseName string,
) error {
	formatted, err := pal.FormatTLAModule(module, pal.TLAFormatOptions{ExtendedModules: extends})
	if err != nil {
		return fmt.Errorf("format frontend intermediate TLA+: %w", err)
	}
	encoded, err := pal.MarshalSingleModuleIRJSON(module, pal.IRJSONOptions{SourceStore: sources})
	if err != nil {
		return fmt.Errorf("format frontend intermediate JSON: %w", err)
	}
	for extension, data := range map[string][]byte{".tla": []byte(formatted), ".json": encoded} {
		files, pathErr := paths.intermediateFiles(baseName + extension)
		if pathErr != nil {
			return pathErr
		}
		for _, path := range files {
			if err = os.WriteFile(path, data, 0o644); err != nil {
				return fmt.Errorf("write frontend intermediate to %s: %w", path, err)
			}
		}
	}
	return nil
}
