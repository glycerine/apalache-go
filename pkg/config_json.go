package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ParseApalacheConfigJSON(text string, sourceName ...string) ConfigParseResult[ApalacheConfig] {
	name := "<configuration>"
	if len(sourceName) > 0 && sourceName[0] != "" {
		name = sourceName[0]
	}
	data := []byte(text)
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ConfigFailureMessage[ApalacheConfig](name + ": Expected a JSON object, but the document is empty.")
	}
	if err := rejectConfigDuplicateKeys(data); err != nil {
		return ConfigFailureMessage[ApalacheConfig](fmt.Sprintf("%s: Invalid strict JSON: %v", name, err))
	}
	if trimmed[0] != '{' {
		probe := json.NewDecoder(bytes.NewReader(data))
		var value any
		if err := probe.Decode(&value); err != nil {
			return ConfigFailureMessage[ApalacheConfig](fmt.Sprintf("%s: Invalid strict JSON: %v", name, err))
		}
		var extra any
		if err := probe.Decode(&extra); err != io.EOF {
			if err == nil {
				return ConfigFailureMessage[ApalacheConfig](name + ": Trailing content after the JSON document is not allowed.")
			}
			return ConfigFailureMessage[ApalacheConfig](fmt.Sprintf("%s: Invalid strict JSON: %v", name, err))
		}
		return ConfigFailureMessage[ApalacheConfig](name + ": Expected the configuration root to be a JSON object.")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]json.RawMessage
	if err := decoder.Decode(&root); err != nil {
		if err == io.EOF {
			return ConfigFailureMessage[ApalacheConfig](name + ": Expected a JSON object, but the document is empty.")
		}
		return ConfigFailureMessage[ApalacheConfig](fmt.Sprintf("%s: Invalid strict JSON: %v", name, err))
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return ConfigFailureMessage[ApalacheConfig](name + ": Trailing content after the JSON document is not allowed.")
		}
		return ConfigFailureMessage[ApalacheConfig](fmt.Sprintf("%s: Invalid strict JSON: %v", name, err))
	}
	parsed := configJSONDecoder{errors: make([]string, 0)}
	config := parsed.decode(root)
	if len(parsed.errors) != 0 {
		return ConfigFailure[ApalacheConfig](parsed.errors)
	}
	return ConfigSuccess(config)
}

type configJSONDecoder struct{ errors []string }

func (decoder *configJSONDecoder) decode(root map[string]json.RawMessage) ApalacheConfig {
	decoder.rejectUnknown(root, "$", []string{
		ConfigCommand, ConfigFile, ConfigOutDir, ConfigRunDir, ConfigDebug, ConfigSMTProf, ConfigWriteIntermediate,
		ConfigProfiling, ConfigFeatures, ConfigSource, ConfigOutput, ConfigChecker, ConfigTypechecker, ConfigTracee, ConfigServer,
	})
	config := ApalacheConfig{}
	config.Context.Command = decoder.text(root, ConfigCommand, "$")
	config.Context.ConfigFile = decoder.path(root, ConfigFile, "$")
	config.Common.OutDir = decoder.path(root, ConfigOutDir, "$")
	config.Common.RunDir = decoder.path(root, ConfigRunDir, "$")
	config.Common.Debug = decoder.boolean(root, ConfigDebug, "$")
	config.Common.SMTProfiling = decoder.boolean(root, ConfigSMTProf, "$")
	config.Common.WriteIntermediate = decoder.boolean(root, ConfigWriteIntermediate, "$")
	config.Common.Profiling = decoder.boolean(root, ConfigProfiling, "$")
	if values := decoder.stringList(root, ConfigFeatures, "$"); values != nil {
		features := make([]Feature, 0, len(*values))
		for _, name := range *values {
			feature, found := FeatureFromString(name)
			if !found {
				decoder.errors = append(decoder.errors, "$.features: Unexpected feature: "+name)
			} else {
				features = append(features, feature)
			}
		}
		config.Common.Features = &features
	}
	config.Source = decoder.source(root, ConfigSource, "$")
	config.Output = decoder.path(root, ConfigOutput, "$")
	config.Checker = decoder.checker(root)
	config.Typechecker = decoder.typechecker(root)
	config.TraceEvaluation = decoder.trace(root)
	config.Server = decoder.server(root)
	return config
}

func (decoder *configJSONDecoder) checker(root map[string]json.RawMessage) CheckerPatch {
	object, found := decoder.object(root, ConfigChecker, "$")
	if !found {
		return CheckerPatch{}
	}
	path := "$.checker"
	decoder.rejectUnknown(object, path, []string{
		ConfigTuning, ConfigAlgorithm, ConfigConfig, ConfigDiscardDisabled, ConfigCInit, ConfigInit, ConfigInvariant,
		ConfigNext, ConfigLength, ConfigMaxError, ConfigTimeoutSMT, ConfigNoDeadlock, ConfigSMTSolver, ConfigSMTEncoding,
		ConfigTemporal, ConfigView,
	})
	patch := CheckerPatch{
		Tuning: decoder.stringMap(object, ConfigTuning, path), TLCConfig: decoder.path(object, ConfigConfig, path),
		DiscardDisabled:     decoder.boolean(object, ConfigDiscardDisabled, path),
		ConstantInitializer: decoder.text(object, ConfigCInit, path), Init: decoder.text(object, ConfigInit, path),
		Invariants: decoder.stringList(object, ConfigInvariant, path), Next: decoder.text(object, ConfigNext, path),
		Length: decoder.integer(object, ConfigLength, path), MaxError: decoder.integer(object, ConfigMaxError, path),
		TimeoutSMTSeconds:  decoder.integer(object, ConfigTimeoutSMT, path),
		TemporalProperties: decoder.stringList(object, ConfigTemporal, path), View: decoder.text(object, ConfigView, path),
	}
	if noDeadlock := decoder.boolean(object, ConfigNoDeadlock, path); noDeadlock != nil {
		value := !*noDeadlock
		patch.CheckDeadlocks = &value
	}
	if value := decoder.text(object, ConfigAlgorithm, path); value != nil {
		parsed, err := ParseCheckerAlgorithm(*value)
		if err != nil {
			decoder.errors = append(decoder.errors, path+".algo: "+err.Error())
		} else {
			patch.Algorithm = &parsed
		}
	}
	if value := decoder.text(object, ConfigSMTSolver, path); value != nil {
		parsed, err := ParseSMTSolverBackend(*value)
		if err != nil {
			decoder.errors = append(decoder.errors, path+".smt-solver: "+err.Error())
		} else {
			patch.SMTSolver = &parsed
		}
	}
	if value := decoder.text(object, ConfigSMTEncoding, path); value != nil {
		parsed, err := ParseSMTEncoding(*value)
		if err != nil {
			decoder.errors = append(decoder.errors, path+".smt-encoding: "+err.Error())
		} else {
			patch.SMTEncoding = &parsed
		}
	}
	return patch
}

func (decoder *configJSONDecoder) typechecker(root map[string]json.RawMessage) TypecheckerPatch {
	object, found := decoder.object(root, ConfigTypechecker, "$")
	if !found {
		return TypecheckerPatch{}
	}
	decoder.rejectUnknown(object, "$.typechecker", []string{ConfigInferPoly})
	return TypecheckerPatch{InferPoly: decoder.boolean(object, ConfigInferPoly, "$.typechecker")}
}

func (decoder *configJSONDecoder) trace(root map[string]json.RawMessage) TraceEvaluationPatch {
	object, found := decoder.object(root, ConfigTracee, "$")
	if !found {
		return TraceEvaluationPatch{}
	}
	decoder.rejectUnknown(object, "$.tracee", []string{ConfigTrace, ConfigExpressions})
	return TraceEvaluationPatch{
		Trace:       decoder.source(object, ConfigTrace, "$.tracee"),
		Expressions: decoder.stringList(object, ConfigExpressions, "$.tracee"),
	}
}

func (decoder *configJSONDecoder) server(root map[string]json.RawMessage) ServerPatch {
	object, found := decoder.object(root, ConfigServer, "$")
	if !found {
		return ServerPatch{}
	}
	path := "$.server"
	decoder.rejectUnknown(object, path, []string{ConfigPort, ConfigServerType})
	patch := ServerPatch{Port: decoder.integer(object, ConfigPort, path)}
	if value := decoder.text(object, ConfigServerType, path); value != nil {
		parsed, err := ParseServerType(*value)
		if err != nil {
			decoder.errors = append(decoder.errors, path+".server-type: "+err.Error())
		} else {
			patch.ServerType = &parsed
		}
	}
	return patch
}

func (decoder *configJSONDecoder) source(object map[string]json.RawMessage, field, parent string) InputSource {
	raw, found := object[field]
	if !found {
		return nil
	}
	path := parent + "." + field
	var filename string
	if err := json.Unmarshal(raw, &filename); err == nil {
		expanded, ok := decoder.expandPath(filename, path)
		if !ok {
			return nil
		}
		value, err := NewFileInputSource(expanded)
		if err != nil {
			decoder.errors = append(decoder.errors, path+": "+capitalizeFirst(err.Error()))
			return nil
		}
		return value
	}
	var sourceObject map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sourceObject); err != nil {
		decoder.errors = append(decoder.errors, path+": Expected a path string or source object.")
		return nil
	}
	decoder.rejectUnknown(sourceObject, path, []string{ConfigKind, ConfigPath, ConfigContent, ConfigAux, ConfigFormat})
	kind := ""
	if value := decoder.text(sourceObject, ConfigKind, path); value != nil {
		kind = strings.ToLower(*value)
	} else if _, found := sourceObject[ConfigContent]; found {
		kind = ConfigStringSource
	} else if _, found := sourceObject[ConfigPath]; found {
		kind = ConfigFileSource
	} else {
		decoder.errors = append(decoder.errors, path+": Source object requires kind, path, or content.")
		return nil
	}
	if kind != ConfigFileSource && kind != ConfigStringSource {
		decoder.errors = append(decoder.errors, fmt.Sprintf("%s.kind: Expected \"file\" or \"string\", but got \"%s\".", path, kind))
		return nil
	}
	format, formatSet := InputFormatTLA, false
	if value := decoder.text(sourceObject, ConfigFormat, path); value != nil {
		parsed, err := ParseInputFormat(*value)
		if err != nil {
			decoder.errors = append(decoder.errors, path+".format: "+capitalizeFirst(err.Error()))
			return nil
		}
		format, formatSet = parsed, true
	}
	if kind == ConfigFileSource {
		filename := decoder.text(sourceObject, ConfigPath, path)
		if filename == nil {
			return nil
		}
		expanded, ok := decoder.expandPath(*filename, path+".path")
		if !ok {
			return nil
		}
		if formatSet {
			return NewFileInputSourceWithFormat(expanded, format)
		}
		value, err := NewFileInputSource(expanded)
		if err != nil {
			decoder.errors = append(decoder.errors, path+": "+capitalizeFirst(err.Error()))
			return nil
		}
		return value
	}
	content := decoder.text(sourceObject, ConfigContent, path)
	aux := decoder.stringList(sourceObject, ConfigAux, path)
	if content == nil {
		return nil
	}
	auxiliary := []string(nil)
	if aux != nil {
		auxiliary = *aux
	}
	return NewStringInputSource(*content, auxiliary, format)
}

func (decoder *configJSONDecoder) text(object map[string]json.RawMessage, field, parent string) *string {
	raw, found := object[field]
	if !found {
		return nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		decoder.errors = append(decoder.errors, parent+"."+field+": Expected a JSON string.")
		return nil
	}
	return &value
}

func (decoder *configJSONDecoder) path(object map[string]json.RawMessage, field, parent string) *string {
	value := decoder.text(object, field, parent)
	if value == nil {
		return nil
	}
	expanded, ok := decoder.expandPath(*value, parent+"."+field)
	if !ok {
		return nil
	}
	return &expanded
}

func (decoder *configJSONDecoder) expandPath(value, path string) (string, bool) {
	if strings.IndexByte(value, 0) >= 0 {
		decoder.errors = append(decoder.errors, path+": Invalid path: Nul character not allowed")
		return "", false
	}
	if value != "~" && !strings.HasPrefix(value, "~/") && !strings.HasPrefix(value, `~\`) {
		return value, true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		decoder.errors = append(decoder.errors, path+": Invalid path: user home is unavailable")
		return "", false
	}
	if value == "~" {
		return home, true
	}
	return filepath.Join(home, value[2:]), true
}

func (decoder *configJSONDecoder) boolean(object map[string]json.RawMessage, field, parent string) *bool {
	raw, found := object[field]
	if !found {
		return nil
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		decoder.errors = append(decoder.errors, parent+"."+field+": Expected a JSON boolean.")
		return nil
	}
	return &value
}

func (decoder *configJSONDecoder) integer(object map[string]json.RawMessage, field, parent string) *int {
	raw, found := object[field]
	if !found {
		return nil
	}
	var leafValue any
	leaf := json.NewDecoder(bytes.NewReader(raw))
	leaf.UseNumber()
	if err := leaf.Decode(&leafValue); err != nil {
		decoder.errors = append(decoder.errors, parent+"."+field+": Expected a 32-bit JSON integer.")
		return nil
	}
	number, ok := leafValue.(json.Number)
	if !ok {
		decoder.errors = append(decoder.errors, parent+"."+field+": Expected a 32-bit JSON integer.")
		return nil
	}
	value, err := number.Int64()
	if err != nil || value < math.MinInt32 || value > math.MaxInt32 {
		decoder.errors = append(decoder.errors, parent+"."+field+": Expected a 32-bit JSON integer.")
		return nil
	}
	result := int(value)
	return &result
}

func (decoder *configJSONDecoder) stringList(object map[string]json.RawMessage, field, parent string) *[]string {
	raw, found := object[field]
	if !found {
		return nil
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		decoder.errors = append(decoder.errors, parent+"."+field+": Expected an array of strings.")
		return nil
	}
	result := make([]string, 0, len(values))
	for index, rawValue := range values {
		var value string
		if err := json.Unmarshal(rawValue, &value); err != nil {
			decoder.errors = append(decoder.errors, fmt.Sprintf("%s.%s[%d]: Expected a JSON string.", parent, field, index))
		} else {
			result = append(result, value)
		}
	}
	return &result
}

func (decoder *configJSONDecoder) stringMap(object map[string]json.RawMessage, field, parent string) *map[string]string {
	raw, found := object[field]
	if !found {
		return nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		decoder.errors = append(decoder.errors, parent+"."+field+": Expected a JSON object with string values.")
		return nil
	}
	result := make(map[string]string, len(values))
	keys := sortedRawKeys(values)
	for _, key := range keys {
		var value string
		if err := json.Unmarshal(values[key], &value); err != nil {
			decoder.errors = append(decoder.errors, parent+"."+field+"."+key+": Expected a JSON string.")
		} else {
			result[key] = value
		}
	}
	return &result
}

func (decoder *configJSONDecoder) object(object map[string]json.RawMessage, field, parent string) (map[string]json.RawMessage, bool) {
	raw, found := object[field]
	if !found {
		return nil, false
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil {
		decoder.errors = append(decoder.errors, parent+"."+field+": Expected a JSON object.")
		return nil, false
	}
	return result, true
}

func (decoder *configJSONDecoder) rejectUnknown(object map[string]json.RawMessage, path string, allowed []string) {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	unknown := make([]string, 0)
	for key := range object {
		if _, found := allowedSet[key]; !found {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		decoder.errors = append(decoder.errors, path+"."+key+": Unknown configuration key.")
	}
}

func sortedRawKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func capitalizeFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func MarshalApalacheConfigJSON(config ApalacheConfig, pretty bool) (string, error) {
	root := make(map[string]any)
	putConfigPointer(root, ConfigCommand, config.Context.Command)
	putConfigPointer(root, ConfigFile, config.Context.ConfigFile)
	putConfigPointer(root, ConfigOutDir, config.Common.OutDir)
	putConfigPointer(root, ConfigRunDir, config.Common.RunDir)
	putConfigPointer(root, ConfigDebug, config.Common.Debug)
	putConfigPointer(root, ConfigSMTProf, config.Common.SMTProfiling)
	putConfigPointer(root, ConfigWriteIntermediate, config.Common.WriteIntermediate)
	putConfigPointer(root, ConfigProfiling, config.Common.Profiling)
	if config.Common.Features != nil {
		features := make([]string, len(*config.Common.Features))
		for index, feature := range *config.Common.Features {
			features[index] = feature.String()
		}
		root[ConfigFeatures] = features
	}
	if config.Source != nil {
		root[ConfigSource] = encodeConfigSource(config.Source)
	}
	putConfigPointer(root, ConfigOutput, config.Output)
	checker := make(map[string]any)
	if config.Checker.Tuning != nil {
		checker[ConfigTuning] = cloneMap(*config.Checker.Tuning)
	}
	putConfigPointer(checker, ConfigAlgorithm, config.Checker.Algorithm)
	putConfigPointer(checker, ConfigConfig, config.Checker.TLCConfig)
	putConfigPointer(checker, ConfigDiscardDisabled, config.Checker.DiscardDisabled)
	putConfigPointer(checker, ConfigCInit, config.Checker.ConstantInitializer)
	putConfigPointer(checker, ConfigInit, config.Checker.Init)
	putConfigPointer(checker, ConfigInvariant, config.Checker.Invariants)
	putConfigPointer(checker, ConfigNext, config.Checker.Next)
	putConfigPointer(checker, ConfigLength, config.Checker.Length)
	putConfigPointer(checker, ConfigMaxError, config.Checker.MaxError)
	putConfigPointer(checker, ConfigTimeoutSMT, config.Checker.TimeoutSMTSeconds)
	if config.Checker.CheckDeadlocks != nil {
		checker[ConfigNoDeadlock] = !*config.Checker.CheckDeadlocks
	}
	putConfigPointer(checker, ConfigSMTSolver, config.Checker.SMTSolver)
	putConfigPointer(checker, ConfigSMTEncoding, config.Checker.SMTEncoding)
	putConfigPointer(checker, ConfigTemporal, config.Checker.TemporalProperties)
	putConfigPointer(checker, ConfigView, config.Checker.View)
	if len(checker) != 0 {
		root[ConfigChecker] = checker
	}
	typechecker := make(map[string]any)
	putConfigPointer(typechecker, ConfigInferPoly, config.Typechecker.InferPoly)
	if len(typechecker) != 0 {
		root[ConfigTypechecker] = typechecker
	}
	trace := make(map[string]any)
	if config.TraceEvaluation.Trace != nil {
		trace[ConfigTrace] = encodeConfigSource(config.TraceEvaluation.Trace)
	}
	putConfigPointer(trace, ConfigExpressions, config.TraceEvaluation.Expressions)
	if len(trace) != 0 {
		root[ConfigTracee] = trace
	}
	server := make(map[string]any)
	putConfigPointer(server, ConfigPort, config.Server.Port)
	putConfigPointer(server, ConfigServerType, config.Server.ServerType)
	if len(server) != 0 {
		root[ConfigServer] = server
	}
	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(root, "", "  ")
	} else {
		data, err = json.Marshal(root)
	}
	return string(data), err
}

func putConfigPointer[T any](object map[string]any, field string, value *T) {
	if value != nil {
		object[field] = *value
	}
}

func encodeConfigSource(source InputSource) any {
	switch value := source.(type) {
	case FileInputSource:
		if inferred, err := NewFileInputSource(value.Path); err == nil && inferred.Format() == value.Format() {
			return value.Path
		}
		return map[string]any{ConfigKind: ConfigFileSource, ConfigPath: value.Path, ConfigFormat: string(value.Format())}
	case *FileInputSource:
		if value == nil {
			return nil
		}
		return encodeConfigSource(*value)
	case StringInputSource:
		return map[string]any{ConfigKind: ConfigStringSource, ConfigContent: value.Content,
			ConfigAux: append([]string(nil), value.Auxiliary...), ConfigFormat: string(value.Format())}
	case *StringInputSource:
		if value == nil {
			return nil
		}
		return encodeConfigSource(*value)
	default:
		return source.String()
	}
}

func rejectConfigDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanConfigJSONValue(decoder, "$"); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	}
	return nil
}

func scanConfigJSONValue(decoder *json.Decoder, path string) error {
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
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key at %s is not a string", path)
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate key %q at %s", key, path)
			}
			seen[key] = struct{}{}
			if err := scanConfigJSONValue(decoder, path+"."+key); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for index := 0; decoder.More(); index++ {
			if err := scanConfigJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}
