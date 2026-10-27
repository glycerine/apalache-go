package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
	Source            *string               `json:"source"`
	Output            *string               `json:"output"`
	Checker           jsonCheckerConfig     `json:"checker"`
	Typechecker       jsonTypecheckerConfig `json:"typechecker"`
	TraceEvaluation   jsonTraceConfig       `json:"tracee"`
	Server            jsonServerConfig      `json:"server"`
}

type jsonTraceConfig struct {
	Trace       json.RawMessage `json:"trace"`
	Expressions *[]string       `json:"expressions"`
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

func applyJSONFrontendDefaults(
	config jsonCommandConfig,
	visited map[string]bool,
	output, features *string,
	useRows, disablePolytypes *bool,
) error {
	if !visited["output"] && config.Output != nil {
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
	if !visited["algo"] && checker.Algorithm != nil {
		targets.backend.algorithm = *checker.Algorithm
	}
	if !visited["smt-encoding"] && checker.SMTEncoding != nil {
		targets.backend.encoding = *checker.SMTEncoding
	}
	if !visited["smt-solver"] && checker.SMTSolver != nil {
		targets.backend.solver = *checker.SMTSolver
	}
	if !visited["length"] && checker.Length != nil {
		*targets.length = *checker.Length
	}
	if !visited["timeout-smt"] && checker.TimeoutSMT != nil {
		*targets.timeoutSMT = *checker.TimeoutSMT
	}
	if !visited["config"] && checker.TLCConfig != nil {
		path, _, err := expandHomePath(*checker.TLCConfig)
		if err != nil {
			return err
		}
		*targets.TLCConfig = path
	}
	if !visited["cinit"] && checker.ConstantInit != nil {
		*targets.constantInit = *checker.ConstantInit
	}
	if !visited["init"] && checker.Init != nil {
		*targets.init = *checker.Init
	}
	if !visited["next"] && checker.Next != nil {
		*targets.next = *checker.Next
	}
	if targets.invariants != nil && !visited["inv"] && checker.Invariants != nil {
		*targets.invariants = append((*targets.invariants)[:0], (*checker.Invariants)...)
	}
	if targets.temporalProperties != nil && !visited["temporal"] && checker.TemporalProperties != nil {
		*targets.temporalProperties = append((*targets.temporalProperties)[:0], (*checker.TemporalProperties)...)
	}
	if !visited["no-polytypes"] && config.Typechecker.InferPoly != nil {
		*targets.disablePolytypes = !*config.Typechecker.InferPoly
	}
	if !visited["rows"] && !visited["features"] && config.Features != nil {
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
