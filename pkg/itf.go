package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
)

const (
	ITFMetaField           = "#meta"
	ITFVarTypesField       = "varTypes"
	ITFVarsField           = "vars"
	ITFParamsField         = "params"
	ITFStatesField         = "states"
	ITFBigIntField         = "#bigint"
	ITFTupleField          = "#tup"
	ITFSetField            = "#set"
	ITFMapField            = "#map"
	ITFUnserializableField = "#unserializable"
)

type TraceState map[string]Expr
type ITFOptions struct {
	Description      string
	NameReplacements map[string]string
}

// MarshalDecodedTraceITF adapts executor states, which already contain both
// constants and variables, to ITF's separate constant-initialization slot.
func MarshalDecodedTraceITF(module *Module, states []DecodedState, options ITFOptions) ([]byte, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot encode decoded trace for nil module")
	}
	if len(states) == 0 {
		return nil, fmt.Errorf("expected at least one decoded state, found none")
	}
	constantNames := make(map[string]struct{}, len(module.ConstDeclarations()))
	for _, declaration := range module.ConstDeclarations() {
		constantNames[declaration.Name()] = struct{}{}
	}
	internalNames := internalVariableNames(module)
	trace := make([]TraceState, 0, len(states)+1)
	constants := make(TraceState)
	for name, value := range states[0].Values {
		if _, constant := constantNames[name]; constant {
			constants[name] = value
		}
	}
	trace = append(trace, constants)
	for _, state := range states {
		values := make(TraceState)
		for name, value := range state.Values {
			if _, internal := internalNames[name]; internal {
				continue
			}
			if _, constant := constantNames[name]; !constant {
				values[name] = value
			}
		}
		trace = append(trace, values)
	}
	return MarshalITFTrace(module, trace, options)
}

func MarshalITFTrace(module *Module, states []TraceState, options ITFOptions) ([]byte, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot encode trace for nil module")
	}
	if len(states) == 0 {
		return nil, fmt.Errorf("expected at least one state, found none")
	}
	stateZero := cloneTraceState(states[0])
	if len(states) > 1 {
		for name, value := range states[1] {
			stateZero[name] = value
		}
	}
	mapped := make([]TraceState, 0, max(1, len(states)-1))
	mapped = append(mapped, stateZero)
	if len(states) > 2 {
		mapped = append(mapped, states[2:]...)
	}
	varTypes := make(map[string]string)
	variables := make([]string, 0)
	parameters := make([]string, 0)
	for _, declaration := range module.Declarations() {
		switch typed := declaration.(type) {
		case *VarDecl:
			if typed.IsInternal() {
				continue
			}
			typ, ok := typed.Tag().Type()
			if !ok {
				return nil, fmt.Errorf("variable %s has no type annotation", typed.Name())
			}
			variables = append(variables, typed.Name())
			varTypes[typed.Name()] = typ.String()
		case *ConstDecl:
			parameters = append(parameters, typed.Name())
		}
	}
	meta := map[string]any{"format": "ITF", "format-description": "https://apalache-mc.org/docs/adr/015adr-trace.html", "varTypes": varTypes}
	if options.Description == "" {
		options.Description = "Created by Apalache Go"
	}
	meta["description"] = options.Description
	replacements := module.NameReplacements()
	for name, expression := range options.NameReplacements {
		replacements[name] = expression
	}
	if len(replacements) > 0 {
		meta["variables-to-expressions"] = replacements
	}
	root := map[string]any{ITFMetaField: meta, ITFVarsField: variables}
	if len(parameters) > 0 {
		root[ITFParamsField] = parameters
	}
	encodedStates := make([]any, len(mapped))
	for index, state := range mapped {
		encoded, err := encodeITFState(state, index)
		if err != nil {
			return nil, fmt.Errorf("state %d: %w", index, err)
		}
		encodedStates[index] = encoded
	}
	root[ITFStatesField] = encodedStates
	return json.Marshal(root)
}

func internalVariableNames(module *Module) map[string]struct{} {
	result := make(map[string]struct{})
	if module == nil {
		return result
	}
	for _, variable := range module.VarDeclarations() {
		if variable.IsInternal() {
			result[variable.Name()] = struct{}{}
		}
	}
	return result
}

// UserDecodedStates removes generated checker state while preserving all
// user-declared constants and variables.
func UserDecodedStates(module *Module, states []DecodedState) []DecodedState {
	internal := internalVariableNames(module)
	result := make([]DecodedState, len(states))
	for index, state := range states {
		values := make(map[string]Expr, len(state.Values))
		for name, value := range state.Values {
			if _, hidden := internal[name]; !hidden {
				values[name] = value
			}
		}
		result[index] = DecodedState{Values: values, TransitionNumber: state.TransitionNumber}
	}
	return result
}

func cloneTraceState(state TraceState) TraceState {
	result := make(TraceState, len(state))
	for name, value := range state {
		result[name] = value
	}
	return result
}
func encodeITFState(state TraceState, index int) (map[string]any, error) {
	result := map[string]any{ITFMetaField: map[string]any{"index": index}}
	names := make([]string, 0, len(state))
	for name := range state {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value, err := EncodeITFValue(state[name])
		if err != nil {
			return nil, fmt.Errorf("variable %s: %w", name, err)
		}
		result[name] = value
	}
	return result, nil
}

func EncodeITFValue(expression Expr) (any, error) {
	switch typed := expression.(type) {
	case *ValueExpr:
		switch value := typed.Value().(type) {
		case IntValue:
			return map[string]any{ITFBigIntField: value.String()}, nil
		case BoolValue:
			return value.Bool(), nil
		case StringValue:
			return value.Value(), nil
		}
	}
	operator, ok := expression.(*OperExpr)
	if !ok {
		return map[string]any{ITFUnserializableField: debugIRExpression(expression)}, nil
	}
	arguments := operator.Arguments()
	switch operator.Operator().Name() {
	case "TUPLE":
		values := make([]any, len(arguments))
		for i, arg := range arguments {
			encoded, err := EncodeITFValue(arg)
			if err != nil {
				return nil, err
			}
			values[i] = encoded
		}
		if typ, typed := operator.Tag().Type(); typed {
			if _, sequence := typ.(SeqType); sequence {
				return values, nil
			}
		}
		return map[string]any{ITFTupleField: values}, nil
	case "SET_ENUM":
		values := make([]any, len(arguments))
		for i, arg := range arguments {
			encoded, err := EncodeITFValue(arg)
			if err != nil {
				return nil, err
			}
			values[i] = encoded
		}
		return map[string]any{ITFSetField: values}, nil
	case "RECORD":
		if len(arguments)%2 != 0 {
			return nil, fmt.Errorf("record has odd argument count")
		}
		record := make(map[string]any, len(arguments)/2)
		for i := 0; i < len(arguments); i += 2 {
			name, ok := arguments[i].(*ValueExpr)
			if !ok {
				return nil, fmt.Errorf("record key %d is not a value", i/2)
			}
			key, ok := name.Value().(StringValue)
			if !ok {
				return nil, fmt.Errorf("record key %d is not a string", i/2)
			}
			encoded, err := EncodeITFValue(arguments[i+1])
			if err != nil {
				return nil, err
			}
			record[key.Value()] = encoded
		}
		return record, nil
	case "Variants!Variant":
		if len(arguments) == 2 {
			tagExpr, ok := arguments[0].(*ValueExpr)
			if ok {
				tag, ok := tagExpr.Value().(StringValue)
				if ok {
					value, err := EncodeITFValue(arguments[1])
					if err != nil {
						return nil, err
					}
					return map[string]any{"tag": tag.Value(), "value": value}, nil
				}
			}
		}
	case "Apalache!SetAsFun":
		if len(arguments) == 1 {
			set, ok := arguments[0].(*OperExpr)
			if ok && set.Operator().Name() == "SET_ENUM" {
				entries := make([]any, 0, len(set.Arguments()))
				for _, entry := range set.Arguments() {
					pair, ok := entry.(*OperExpr)
					if !ok || pair.Operator().Name() != "TUPLE" || len(pair.Arguments()) != 2 {
						continue
					}
					key, err := EncodeITFValue(pair.Arguments()[0])
					if err != nil {
						return nil, err
					}
					value, err := EncodeITFValue(pair.Arguments()[1])
					if err != nil {
						return nil, err
					}
					entries = append(entries, []any{key, value})
				}
				return map[string]any{ITFMapField: entries}, nil
			}
		}
	}
	return map[string]any{ITFUnserializableField: debugIRExpression(expression)}, nil
}

func debugIRExpression(expression Expr) string {
	switch typed := expression.(type) {
	case *NameExpr:
		return typed.Name()
	case *ValueExpr:
		return fmt.Sprint(typed.Value())
	case *OperExpr:
		parts := make([]string, len(typed.Arguments()))
		for i, arg := range typed.Arguments() {
			parts[i] = debugIRExpression(arg)
		}
		return typed.Operator().Name() + "(" + joinStrings(parts, ",") + ")"
	case *LetInExpr:
		return "LET_IN"
	default:
		return "NullEx"
	}
}
func joinStrings(values []string, separator string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += separator
		}
		result += value
	}
	return result
}

type ITFError struct{ Kind, Message string }

func (err *ITFError) Error() string { return err.Kind + ": " + err.Message }
func itfFormat(format string, args ...any) error {
	return &ITFError{Kind: "ITF format error", Message: fmt.Sprintf(format, args...)}
}
func itfContent(format string, args ...any) error {
	return &ITFError{Kind: "ITF content error", Message: fmt.Sprintf(format, args...)}
}

func UnmarshalITFTrace(data []byte) ([]TraceState, error) {
	return unmarshalITFTrace(data, nil)
}

// UnmarshalITFTraceWithParams decodes an ITF trace whose params receive their
// types from the source module. ITF deliberately records varTypes only for
// vars; parameter values occur in state zero and are constant thereafter.
func UnmarshalITFTraceWithParams(data []byte, parameterTypes map[string]Type) ([]TraceState, error) {
	return unmarshalITFTrace(data, parameterTypes)
}

func unmarshalITFTrace(data []byte, parameterTypes map[string]Type) ([]TraceState, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, itfFormat("invalid JSON: %v", err)
	}
	meta, ok := root[ITFMetaField].(map[string]any)
	if !ok {
		return nil, itfFormat("ITF JSON must have %s with %s", ITFMetaField, ITFVarTypesField)
	}
	rawTypes, ok := meta[ITFVarTypesField].(map[string]any)
	if !ok {
		return nil, itfFormat("%s must contain an object %s", ITFMetaField, ITFVarTypesField)
	}
	rawVars, ok := root[ITFVarsField].([]any)
	if !ok {
		return nil, itfFormat("ITF JSON must declare a %s array", ITFVarsField)
	}
	variables := make([]string, len(rawVars))
	variableSet := make(map[string]struct{}, len(rawVars))
	for i, raw := range rawVars {
		name, ok := raw.(string)
		if !ok {
			return nil, itfFormat("variable name %d is not a string", i)
		}
		variables[i] = name
		variableSet[name] = struct{}{}
	}
	for _, name := range variables {
		if _, ok := rawTypes[name]; !ok {
			return nil, itfFormat("missing type annotation for %s", name)
		}
	}
	for name := range rawTypes {
		if _, ok := variableSet[name]; !ok {
			return nil, itfFormat("type annotation for unknown variable %s", name)
		}
	}
	parameters := make([]string, 0)
	parameterSet := make(map[string]struct{})
	if rawParams, found := root[ITFParamsField]; found {
		array, ok := rawParams.([]any)
		if !ok {
			return nil, itfFormat("%s must be an array", ITFParamsField)
		}
		parameters = make([]string, len(array))
		for index, raw := range array {
			name, ok := raw.(string)
			if !ok {
				return nil, itfFormat("parameter name %d is not a string", index)
			}
			if _, variable := variableSet[name]; variable {
				return nil, itfContent("%s is declared as both a variable and parameter", name)
			}
			if _, duplicate := parameterSet[name]; duplicate {
				return nil, itfContent("parameter %s is repeated", name)
			}
			if parameterTypes[name] == nil {
				return nil, itfContent("missing source type for parameter %s", name)
			}
			parameters[index] = name
			parameterSet[name] = struct{}{}
		}
	}
	types := make(map[string]Type, len(rawTypes))
	for name, raw := range rawTypes {
		text, ok := raw.(string)
		if !ok {
			return nil, itfFormat("type of %s is not a string", name)
		}
		typ, err := ParseType(text)
		if err != nil {
			return nil, itfContent("invalid type of %s: %v", name, err)
		}
		types[name] = typ
	}
	rawStates, ok := root[ITFStatesField].([]any)
	if !ok {
		return nil, itfFormat("ITF JSON must declare a %s array", ITFStatesField)
	}
	states := make([]TraceState, len(rawStates))
	for index, rawState := range rawStates {
		object, ok := rawState.(map[string]any)
		if !ok {
			return nil, itfFormat("state %d must be an object", index)
		}
		for name := range object {
			if name == ITFMetaField {
				continue
			}
			_, variable := variableSet[name]
			_, parameter := parameterSet[name]
			if !variable && !parameter {
				return nil, itfFormat("undeclared variable %s in state %d", name, index)
			}
		}
		state := make(TraceState, len(variables))
		for _, name := range variables {
			rawValue, ok := object[name]
			if !ok {
				return nil, itfFormat("variable %s missing from state %d", name, index)
			}
			value, err := DecodeITFValue(rawValue, types[name])
			if err != nil {
				return nil, fmt.Errorf("state %d variable %s: %w", index, name, err)
			}
			state[name] = value
		}
		for _, name := range parameters {
			rawValue, present := object[name]
			if index == 0 && !present {
				return nil, itfFormat("parameter %s missing from state 0", name)
			}
			if !present {
				continue
			}
			value, err := DecodeITFValue(rawValue, parameterTypes[name])
			if err != nil {
				return nil, fmt.Errorf("state %d parameter %s: %w", index, name, err)
			}
			state[name] = value
		}
		states[index] = state
	}
	if len(states) > 0 {
		for _, name := range parameters {
			value := states[0][name]
			for index := 1; index < len(states); index++ {
				states[index][name] = value
			}
		}
	}
	return states, nil
}

var itfIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func DecodeITFValue(raw any, typ Type) (Expr, error) {
	if object, ok := raw.(map[string]any); ok {
		if len(object) == 1 {
			if value, found := object[ITFUnserializableField]; found {
				return nil, itfContent("unserializable value %v is disallowed", value)
			}
		}
	}
	switch typed := typ.(type) {
	case primitiveType:
		switch typed {
		case BoolT1:
			value, ok := raw.(bool)
			if !ok {
				return nil, itfContent("expected Bool, found %T", raw)
			}
			return NewValueExpr(BoolValue(value), Typed(typ)), nil
		case StrT1:
			value, ok := raw.(string)
			if !ok {
				return nil, itfContent("expected Str, found %T", raw)
			}
			if configModelStringPattern.MatchString(value) {
				return nil, itfContent("%s is annotated as a string but is an uninterpreted literal", value)
			}
			return NewValueExpr(StringValue(value), Typed(typ)), nil
		case IntT1:
			return decodeITFInteger(raw, typ)
		default:
			return nil, itfContent("type %s is not permitted in ITF", typ)
		}
	case ConstType:
		value, ok := raw.(string)
		if !ok {
			return nil, itfContent("expected uninterpreted literal, found %T", raw)
		}
		match := configModelStringPattern.FindStringSubmatch(value)
		if match == nil {
			return nil, itfContent("%s has string form, expected %s literal", value, typed)
		}
		if match[2] != typed.String() {
			return nil, itfContent("%s has type %s, expected %s", value, match[2], typed)
		}
		return NewValueExpr(StringValue(value), Typed(typ)), nil
	case SeqType:
		array, ok := raw.([]any)
		if !ok {
			return nil, itfFormat("sequence must be an array")
		}
		arguments, err := decodeITFArray(array, typed.Element())
		if err != nil {
			return nil, err
		}
		return NewOperExpr(Op("TUPLE"), Typed(typ), arguments...)
	case TupleType:
		object, ok := exactITFObject(raw, ITFTupleField)
		if !ok {
			return nil, itfFormat("tuple must declare only %s", ITFTupleField)
		}
		array, ok := object.([]any)
		if !ok {
			return nil, itfFormat("%s must be an array", ITFTupleField)
		}
		elements := typed.Elements()
		if len(array) != len(elements) {
			return nil, itfContent("tuple value and type have %d and %d elements", len(array), len(elements))
		}
		arguments := make([]Expr, len(array))
		for i := range array {
			value, err := DecodeITFValue(array[i], elements[i])
			if err != nil {
				return nil, err
			}
			arguments[i] = value
		}
		return NewOperExpr(Op("TUPLE"), Typed(typ), arguments...)
	case SetType:
		value, ok := exactITFObject(raw, ITFSetField)
		if !ok {
			return nil, itfFormat("set must declare only %s", ITFSetField)
		}
		array, ok := value.([]any)
		if !ok {
			return nil, itfFormat("%s must be an array", ITFSetField)
		}
		arguments, err := decodeITFArray(array, typed.Element())
		if err != nil {
			return nil, err
		}
		return NewOperExpr(Op("SET_ENUM"), Typed(typ), arguments...)
	case FunType:
		return decodeITFFunction(raw, typed)
	case RecordRowType:
		return decodeITFRecord(raw, typ, typed.Row().Fields())
	case RecordType:
		return decodeITFRecord(raw, typ, typed.Fields())
	case VariantType:
		return decodeITFVariant(raw, typed)
	default:
		return nil, itfContent("type %s does not match a permitted ITF value type", typ)
	}
}

func decodeITFInteger(raw any, typ Type) (Expr, error) {
	var text string
	if object, ok := raw.(map[string]any); ok && len(object) == 1 {
		if value, found := object[ITFBigIntField]; found {
			var stringOK bool
			text, stringOK = value.(string)
			if !stringOK {
				return nil, itfFormat("%s must be a string", ITFBigIntField)
			}
		}
	}
	if text == "" {
		number, ok := raw.(json.Number)
		if !ok {
			return nil, itfContent("expected integer, found %T", raw)
		}
		text = number.String()
	}
	integer, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, itfContent("invalid integer %q", text)
	}
	return NewValueExpr(NewIntValue(integer), Typed(typ)), nil
}
func exactITFObject(raw any, field string) (any, bool) {
	object, ok := raw.(map[string]any)
	if !ok || len(object) != 1 {
		return nil, false
	}
	value, ok := object[field]
	return value, ok
}
func decodeITFArray(array []any, elementType Type) ([]Expr, error) {
	result := make([]Expr, len(array))
	for i, raw := range array {
		value, err := DecodeITFValue(raw, elementType)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		result[i] = value
	}
	return result, nil
}
func decodeITFRecord(raw any, typ Type, fields map[string]Type) (Expr, error) {
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, itfFormat("record must be an object")
	}
	if len(object) == 0 {
		return nil, itfFormat("records may not be empty")
	}
	if len(object) != len(fields) {
		return nil, itfFormat("record fields do not match type")
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if !itfIdentifier.MatchString(key) {
			return nil, itfContent("record key %s is not a valid identifier", key)
		}
		if _, ok := object[key]; !ok {
			return nil, itfFormat("record is missing field %s", key)
		}
		keys = append(keys, key)
	}
	for key := range object {
		if _, ok := fields[key]; !ok {
			return nil, itfFormat("record has spurious field %s", key)
		}
	}
	sort.Strings(keys)
	arguments := make([]Expr, 0, len(keys)*2)
	for _, key := range keys {
		value, err := DecodeITFValue(object[key], fields[key])
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, NewValueExpr(StringValue(key), Typed(StrT1)), value)
	}
	return NewOperExpr(Op("RECORD"), Typed(typ), arguments...)
}
func decodeITFFunction(raw any, typ FunType) (Expr, error) {
	value, ok := exactITFObject(raw, ITFMapField)
	if !ok {
		return nil, itfFormat("function must declare only %s", ITFMapField)
	}
	array, ok := value.([]any)
	if !ok {
		return nil, itfFormat("%s must be an array", ITFMapField)
	}
	pairType, _ := NewTupleType(typ.Argument(), typ.Result())
	pairs := make([]Expr, len(array))
	for i, rawPair := range array {
		pair, ok := rawPair.([]any)
		if !ok || len(pair) != 2 {
			return nil, itfFormat("function entry %d must be a two-element array", i)
		}
		key, err := DecodeITFValue(pair[0], typ.Argument())
		if err != nil {
			return nil, err
		}
		value, err := DecodeITFValue(pair[1], typ.Result())
		if err != nil {
			return nil, err
		}
		pairs[i] = MustOperExpr(Op("TUPLE"), Typed(pairType), key, value)
	}
	setType, _ := NewSetType(pairType)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), pairs...)
	return NewOperExpr(Op("Apalache!SetAsFun"), Typed(typ), set)
}
func decodeITFVariant(raw any, typ VariantType) (Expr, error) {
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, itfFormat("variant must be an object")
	}
	rawTag, tagFound := object["tag"]
	rawValue, valueFound := object["value"]
	if !tagFound || !valueFound {
		return nil, itfFormat("variant must declare tag and value")
	}
	tag, ok := rawTag.(string)
	if !ok {
		return nil, itfFormat("variant tag must be a string")
	}
	tagType, ok := typ.Row().Fields()[tag]
	if !ok {
		return nil, itfContent("tag %q is not present in variant type %s", tag, typ)
	}
	value, err := DecodeITFValue(rawValue, tagType)
	if err != nil {
		return nil, err
	}
	return NewOperExpr(Op("Variants!Variant"), Typed(typ), NewValueExpr(StringValue(tag), Typed(StrT1)), value)
}
