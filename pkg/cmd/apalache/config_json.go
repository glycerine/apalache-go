package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type jsonInputSource struct {
	Kind    *string   `json:"kind"`
	Path    *string   `json:"path"`
	Content *string   `json:"content"`
	Aux     *[]string `json:"aux"`
	Format  *string   `json:"format"`
}

func (source *jsonInputSource) UnmarshalJSON(data []byte) error {
	var path string
	if err := json.Unmarshal(data, &path); err == nil {
		source.Kind = stringPointer("file")
		source.Path = &path
		return nil
	}
	type sourceObject jsonInputSource
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var object sourceObject
	if err := decoder.Decode(&object); err != nil {
		return fmt.Errorf("source must be a path string or source object: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	*source = jsonInputSource(object)
	return source.validate()
}

func stringPointer(value string) *string { return &value }

func (source *jsonInputSource) validate() error {
	if source == nil {
		return nil
	}
	kind := ""
	if source.Kind != nil {
		kind = strings.ToLower(*source.Kind)
	} else if source.Content != nil {
		kind = "string"
	} else if source.Path != nil {
		kind = "file"
	}
	switch kind {
	case "file":
		if source.Path == nil || *source.Path == "" {
			return fmt.Errorf("file source requires a nonempty path")
		}
		if source.Content != nil || source.Aux != nil {
			return fmt.Errorf("file source cannot contain content or aux")
		}
	case "string":
		if source.Content == nil {
			return fmt.Errorf("string source requires content")
		}
		if source.Path != nil {
			return fmt.Errorf("string source cannot contain path")
		}
	default:
		return fmt.Errorf("source kind must be file or string")
	}
	if source.Format != nil && !validSourceFormat(*source.Format) {
		return fmt.Errorf("unsupported source format %q", *source.Format)
	}
	return nil
}

func validSourceFormat(format string) bool {
	switch strings.ToLower(format) {
	case "tla", "json", "itf", "qnt":
		return true
	default:
		return false
	}
}

func inferSourceFormat(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".itf.json"):
		return "itf"
	case strings.HasSuffix(lower, ".qnt.json"):
		return "qnt"
	case strings.HasSuffix(lower, ".json"):
		return "json"
	case strings.HasSuffix(lower, ".tla"):
		return "tla"
	default:
		return ""
	}
}

var moduleNamePattern = regexp.MustCompile(`(?m)\bMODULE[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)

func tlaModuleName(source string) (string, error) {
	match := moduleNamePattern.FindStringSubmatch(source)
	if len(match) != 2 {
		return "", fmt.Errorf("in-memory TLA+ source has no MODULE declaration")
	}
	return match[1], nil
}

func (source *jsonInputSource) materializeSpecification() (string, func(), error) {
	noCleanup := func() {}
	if err := source.validate(); err != nil {
		return "", noCleanup, err
	}
	kind := "file"
	if source.Kind != nil {
		kind = strings.ToLower(*source.Kind)
	} else if source.Content != nil {
		kind = "string"
	}
	format := ""
	if source.Format != nil {
		format = strings.ToLower(*source.Format)
	}
	if kind == "file" {
		path, _, err := expandHomePath(*source.Path)
		if err != nil {
			return "", noCleanup, err
		}
		if format == "" {
			format = inferSourceFormat(path)
		}
		if format == "" {
			return "", noCleanup, fmt.Errorf("cannot infer source format from %q", path)
		}
		if format != "tla" && format != "json" {
			return "", noCleanup, fmt.Errorf("Go specification commands do not support %s sources", format)
		}
		if inferSourceFormat(path) == format {
			return path, noCleanup, nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", noCleanup, err
		}
		materialized, cleanup, err := materializeSourceFiles(string(contents), nil, format)
		if err != nil || format != "tla" {
			return materialized, cleanup, err
		}
		if err = copyAdjacentTLAModules(filepath.Dir(path), filepath.Dir(materialized)); err != nil {
			cleanup()
			return "", noCleanup, err
		}
		return materialized, cleanup, nil
	}
	if format == "" {
		format = "tla"
	}
	if format != "tla" && format != "json" {
		return "", noCleanup, fmt.Errorf("Go specification commands do not support %s sources", format)
	}
	aux := []string(nil)
	if source.Aux != nil {
		aux = *source.Aux
	}
	if format != "tla" && len(aux) != 0 {
		return "", noCleanup, fmt.Errorf("auxiliary modules require TLA+ source format")
	}
	return materializeSourceFiles(*source.Content, aux, format)
}

func (source *jsonInputSource) materializeTrace() (string, func(), error) {
	noCleanup := func() {}
	if err := source.validate(); err != nil {
		return "", noCleanup, err
	}
	kind := "file"
	if source.Kind != nil {
		kind = strings.ToLower(*source.Kind)
	} else if source.Content != nil {
		kind = "string"
	}
	format := ""
	if source.Format != nil {
		format = strings.ToLower(*source.Format)
	}
	if kind == "file" {
		path, _, err := expandHomePath(*source.Path)
		if err != nil {
			return "", noCleanup, err
		}
		if format == "" {
			format = inferSourceFormat(path)
		}
		if format != "itf" {
			return "", noCleanup, fmt.Errorf("Go trace evaluation currently requires an itf trace source")
		}
		if inferSourceFormat(path) == format {
			return path, noCleanup, nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", noCleanup, err
		}
		return materializeTraceFile(contents)
	}
	if source.Aux != nil && len(*source.Aux) != 0 {
		return "", noCleanup, fmt.Errorf("trace source cannot contain auxiliary modules")
	}
	if format != "itf" {
		return "", noCleanup, fmt.Errorf("Go trace evaluation currently requires an itf trace source")
	}
	return materializeTraceFile([]byte(*source.Content))
}

func materializeTraceFile(content []byte) (string, func(), error) {
	directory, err := os.MkdirTemp("", "apalache-trace-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	name := "trace.itf.json"
	path := filepath.Join(directory, name)
	if err = os.WriteFile(path, content, 0o600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}

func copyAdjacentTLAModules(sourceDirectory, targetDirectory string) error {
	entries, err := os.ReadDir(sourceDirectory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".tla") {
			continue
		}
		target := filepath.Join(targetDirectory, entry.Name())
		if _, statErr := os.Stat(target); statErr == nil {
			continue
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		contents, readErr := os.ReadFile(filepath.Join(sourceDirectory, entry.Name()))
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(target, contents, 0o600); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func materializeSourceFiles(content string, aux []string, format string) (string, func(), error) {
	directory, err := os.MkdirTemp("", "apalache-source-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	extension := ".json"
	name := "source"
	if format == "tla" {
		extension = ".tla"
		name, err = tlaModuleName(content)
		if err != nil {
			cleanup()
			return "", func() {}, err
		}
		for _, auxiliary := range aux {
			auxName, nameErr := tlaModuleName(auxiliary)
			if nameErr != nil {
				cleanup()
				return "", func() {}, nameErr
			}
			if writeErr := os.WriteFile(filepath.Join(directory, auxName+extension), []byte(auxiliary), 0o600); writeErr != nil {
				cleanup()
				return "", func() {}, writeErr
			}
		}
	}
	path := filepath.Join(directory, name+extension)
	if err = os.WriteFile(path, []byte(content), 0o600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}

type jsonCheckerConfig struct {
	Tuning             *map[string]string `json:"tuning"`
	Algorithm          *string            `json:"algo"`
	TLCConfig          *string            `json:"config"`
	DiscardDisabled    *bool              `json:"discard-disabled"`
	ConstantInit       *string            `json:"cinit"`
	Init               *string            `json:"init"`
	Invariants         *[]string          `json:"inv"`
	Next               *string            `json:"next"`
	Length             *int               `json:"length"`
	MaxError           *int               `json:"max-error"`
	TimeoutSMT         *int               `json:"timeout-smt"`
	NoDeadlock         *bool              `json:"no-deadlock"`
	SMTSolver          *string            `json:"smt-solver"`
	SMTEncoding        *string            `json:"smt-encoding"`
	TemporalProperties *[]string          `json:"temporal"`
	View               *string            `json:"view"`
}

type jsonTypecheckerConfig struct {
	InferPoly *bool `json:"infer-poly"`
}

type jsonCommandConfig struct {
	Command           *string               `json:"command"`
	ConfigFile        *string               `json:"config-file"`
	OutDir            *string               `json:"out-dir"`
	RunDir            *string               `json:"run-dir"`
	Debug             *bool                 `json:"debug"`
	SMTProfile        *bool                 `json:"smtprof"`
	WriteIntermediate *bool                 `json:"write-intermediate"`
	Profiling         *bool                 `json:"profiling"`
	Features          *[]string             `json:"features"`
	Source            *jsonInputSource      `json:"source"`
	Output            *string               `json:"output"`
	Checker           jsonCheckerConfig     `json:"checker"`
	Typechecker       jsonTypecheckerConfig `json:"typechecker"`
	TraceEvaluation   jsonTraceConfig       `json:"tracee"`
	Server            jsonServerConfig      `json:"server"`
}

type jsonTraceConfig struct {
	Trace       *jsonInputSource `json:"trace"`
	Expressions *[]string        `json:"expressions"`
}

type jsonServerConfig struct {
	Port       *int    `json:"port"`
	ServerType *string `json:"server-type"`
}

func (config jsonCommandConfig) validateForModelCommand(command string, checkSpecific bool) error {
	unsupported := []struct {
		set  bool
		name string
	}{
		{config.Command != nil, "command"},
		{config.ConfigFile != nil, "config-file"},
		{config.OutDir != nil, "out-dir"},
		{config.RunDir != nil, "run-dir"},
		{config.Debug != nil, "debug"},
		{config.SMTProfile != nil, "smtprof"},
		{config.WriteIntermediate != nil, "write-intermediate"},
		{config.Profiling != nil, "profiling"},
		{config.Output != nil, "output"},
	}
	if checkSpecific {
		unsupported = append(unsupported, struct {
			set  bool
			name string
		}{config.Checker.View != nil, "checker.view"})
	}
	for _, field := range unsupported {
		if field.set {
			return fmt.Errorf("JSON configuration field %s is not supported by the Go %s command", field.name, command)
		}
	}
	if config.Checker.Tuning != nil && len(*config.Checker.Tuning) != 0 {
		return fmt.Errorf("JSON checker.tuning is not supported by the g3 backend")
	}
	if config.Features != nil {
		for _, feature := range *config.Features {
			if feature != "rows" {
				return fmt.Errorf("unsupported JSON feature %q (want rows)", feature)
			}
		}
	}
	return nil
}

func (config jsonCommandConfig) validateForCheck() error {
	return config.validateForModelCommand("check", true)
}

func (config jsonCommandConfig) validateForSimulate() error {
	return config.validateForModelCommand("simulate", false)
}

func (config jsonCommandConfig) validateForFrontend(command string) error {
	unsupported := []struct {
		set  bool
		name string
	}{
		{config.Command != nil, "command"},
		{config.ConfigFile != nil, "config-file"},
		{config.OutDir != nil, "out-dir"},
		{config.RunDir != nil, "run-dir"},
		{config.Debug != nil, "debug"},
		{config.SMTProfile != nil, "smtprof"},
		{config.WriteIntermediate != nil, "write-intermediate"},
		{config.Profiling != nil, "profiling"},
	}
	for _, field := range unsupported {
		if field.set {
			return fmt.Errorf("JSON configuration field %s is not supported by the Go %s command", field.name, command)
		}
	}
	if config.Features != nil {
		for _, feature := range *config.Features {
			if feature != "rows" {
				return fmt.Errorf("unsupported JSON feature %q (want rows)", feature)
			}
		}
	}
	return nil
}

func (config jsonCommandConfig) validateForTracee() error {
	if err := config.validateForFrontend("tracee"); err != nil {
		return err
	}
	if config.Output != nil {
		return fmt.Errorf("JSON configuration field output is not supported by the Go tracee command")
	}
	return nil
}

func (config jsonCommandConfig) configuredTracePath() (string, func(), bool, error) {
	noCleanup := func() {}
	if config.TraceEvaluation.Trace == nil {
		return "", noCleanup, false, nil
	}
	path, cleanup, err := config.TraceEvaluation.Trace.materializeTrace()
	return path, cleanup, true, err
}

func applyJSONFrontendDefaults(
	config jsonCommandConfig,
	visited map[string]bool,
	output, features *string,
	useRows, disablePolytypes *bool,
) error {
	if output != nil && !visited["output"] && config.Output != nil {
		path, _, err := expandHomePath(*config.Output)
		if err != nil {
			return err
		}
		*output = path
	}
	if !visited["rows"] && !visited["features"] && config.Features != nil {
		*features = ""
		if useRows != nil {
			*useRows = false
		}
		for _, feature := range *config.Features {
			if feature == "rows" {
				*features = "rows"
				if useRows != nil {
					*useRows = true
				}
			}
		}
	}
	if disablePolytypes != nil && !visited["no-polytypes"] && config.Typechecker.InferPoly != nil {
		*disablePolytypes = !*config.Typechecker.InferPoly
	}
	return nil
}

type jsonModelConfigTargets struct {
	backend            *checkerBackendOptions
	length             *int
	timeoutSMT         *int
	TLCConfig          *string
	constantInit       *string
	init               *string
	next               *string
	useRows            *bool
	disablePolytypes   *bool
	invariants         *csvStringList
	temporalProperties *csvStringList
	enforceMaxError    bool
}

func applyJSONModelDefaults(config jsonCommandConfig, visited map[string]bool, targets jsonModelConfigTargets) error {
	checker := config.Checker
	if targets.backend != nil && !visited["algo"] && checker.Algorithm != nil {
		targets.backend.algorithm = *checker.Algorithm
	}
	if targets.backend != nil && !visited["smt-encoding"] && checker.SMTEncoding != nil {
		targets.backend.encoding = *checker.SMTEncoding
	}
	if targets.backend != nil && !visited["smt-solver"] && checker.SMTSolver != nil {
		targets.backend.solver = *checker.SMTSolver
	}
	if targets.length != nil && !visited["length"] && checker.Length != nil {
		*targets.length = *checker.Length
	}
	if targets.timeoutSMT != nil && !visited["timeout-smt"] && checker.TimeoutSMT != nil {
		*targets.timeoutSMT = *checker.TimeoutSMT
	}
	if targets.TLCConfig != nil && !visited["config"] && checker.TLCConfig != nil {
		path, _, err := expandHomePath(*checker.TLCConfig)
		if err != nil {
			return err
		}
		*targets.TLCConfig = path
	}
	if targets.constantInit != nil && !visited["cinit"] && checker.ConstantInit != nil {
		*targets.constantInit = *checker.ConstantInit
	}
	if targets.init != nil && !visited["init"] && checker.Init != nil {
		*targets.init = *checker.Init
	}
	if targets.next != nil && !visited["next"] && checker.Next != nil {
		*targets.next = *checker.Next
	}
	if targets.invariants != nil && !visited["inv"] && checker.Invariants != nil {
		*targets.invariants = append((*targets.invariants)[:0], (*checker.Invariants)...)
	}
	if targets.temporalProperties != nil && !visited["temporal"] && checker.TemporalProperties != nil {
		*targets.temporalProperties = append((*targets.temporalProperties)[:0], (*checker.TemporalProperties)...)
	}
	if targets.disablePolytypes != nil && !visited["no-polytypes"] && config.Typechecker.InferPoly != nil {
		*targets.disablePolytypes = !*config.Typechecker.InferPoly
	}
	if targets.useRows != nil && !visited["rows"] && !visited["features"] && config.Features != nil {
		*targets.useRows = false
		for _, feature := range *config.Features {
			*targets.useRows = *targets.useRows || feature == "rows"
		}
	}
	if targets.enforceMaxError && checker.MaxError != nil && *checker.MaxError != 1 {
		return fmt.Errorf("JSON checker.max-error currently supports only 1")
	}
	return nil
}

func loadCommandConfig(explicit string) (jsonCommandConfig, string, error) {
	path, required, err := selectCommandConfig(explicit)
	if err != nil || path == "" {
		return jsonCommandConfig{}, path, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !required && os.IsNotExist(err) {
			return jsonCommandConfig{}, "", nil
		}
		return jsonCommandConfig{}, path, err
	}
	if err = rejectDuplicateJSONKeys(data); err != nil {
		return jsonCommandConfig{}, path, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config jsonCommandConfig
	if err = decoder.Decode(&config); err != nil {
		return jsonCommandConfig{}, path, err
	}
	if err = requireJSONEOF(decoder); err != nil {
		return jsonCommandConfig{}, path, err
	}
	return config, path, nil
}

func selectCommandConfig(explicit string) (path string, required bool, err error) {
	if explicit != "" {
		if strings.EqualFold(filepath.Ext(explicit), ".cfg") {
			return "", true, fmt.Errorf("legacy Apalache configuration file %s is not supported; use strict JSON", explicit)
		}
		return expandHomePath(explicit)
	}
	if environment := os.Getenv("CONFIG_FILE"); environment != "" {
		if strings.EqualFold(filepath.Ext(environment), ".cfg") {
			return "", true, fmt.Errorf("legacy Apalache configuration file %s is not supported; use strict JSON", environment)
		}
		path, _, err = expandHomePath(environment)
		return path, true, err
	}
	if _, statErr := os.Stat(".apalache.json"); statErr == nil {
		return ".apalache.json", false, nil
	} else if !os.IsNotExist(statErr) {
		return "", false, statErr
	}
	home, homeErr := os.UserHomeDir()
	if homeErr != nil || home == "" {
		return "", false, nil
	}
	return filepath.Join(home, ".tlaplus", "apalache.json"), false, nil
}

func expandHomePath(path string) (string, bool, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, true, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", true, fmt.Errorf("cannot expand %q without a user home directory", path)
	}
	if path == "~" {
		return home, true, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), true, nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, "$"); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func scanJSONValue(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return keyErr
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key at %s is not a string", path)
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate key %q at %s", key, path)
			}
			seen[key] = struct{}{}
			if err = scanJSONValue(decoder, path+"."+key); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for index := 0; decoder.More(); index++ {
			if err = scanJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("configuration contains a trailing JSON value")
}
