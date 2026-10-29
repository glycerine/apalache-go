package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// JSONRepresentation is the standard-library-backed JSON value used by the
// generic JSON adapters. Its zero value represents JSON null.
type JSONRepresentation struct {
	value any
}

func NewJSONRepresentation(value any) JSONRepresentation {
	return JSONRepresentation{value: value}
}

func ParseJSONRepresentation(data []byte) (JSONRepresentation, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return JSONRepresentation{}, NewJSONDeserializationError("invalid JSON", err)
	}
	return NewJSONRepresentation(value), nil
}

func (representation JSONRepresentation) Value() any { return representation.value }

func (representation JSONRepresentation) String() string {
	data, err := json.MarshalIndent(representation.value, "", "  ")
	if err != nil {
		return "null"
	}
	return string(data)
}

func (representation JSONRepresentation) Field(name string) (JSONRepresentation, bool) {
	object, ok := representation.value.(map[string]any)
	if !ok {
		return JSONRepresentation{}, false
	}
	value, found := object[name]
	if !found {
		return JSONRepresentation{}, false
	}
	return NewJSONRepresentation(value), true
}

func (representation JSONRepresentation) Fields() (map[string]struct{}, bool) {
	object, ok := representation.value.(map[string]any)
	if !ok {
		return nil, false
	}
	fields := make(map[string]struct{}, len(object))
	for name := range object {
		fields[name] = struct{}{}
	}
	return fields, true
}

func JSONObject(fields ...JSONField) JSONRepresentation {
	object := make(map[string]any, len(fields))
	for _, field := range fields {
		object[field.Name] = field.Value.value
	}
	return NewJSONRepresentation(object)
}

type JSONField struct {
	Name  string
	Value JSONRepresentation
}

func JSONInt(value int) JSONRepresentation       { return NewJSONRepresentation(value) }
func JSONString(value string) JSONRepresentation { return NewJSONRepresentation(value) }
func JSONBool(value bool) JSONRepresentation     { return NewJSONRepresentation(value) }
func JSONArray(values ...JSONRepresentation) JSONRepresentation {
	array := make([]any, len(values))
	for index, value := range values {
		array[index] = value.value
	}
	return NewJSONRepresentation(array)
}

func JSONAsInt(value JSONRepresentation) (int, error) {
	switch number := value.value.(type) {
	case int:
		return number, nil
	case json.Number:
		integer, err := number.Int64()
		if err == nil && int64(int(integer)) == integer {
			return int(integer), nil
		}
	}
	return 0, NewJSONDeserializationError("Not an integer.", nil)
}

func JSONAsString(value JSONRepresentation) (string, error) {
	result, ok := value.value.(string)
	if !ok {
		return "", NewJSONDeserializationError("Not a string.", nil)
	}
	return result, nil
}

func JSONAsBool(value JSONRepresentation) (bool, error) {
	result, ok := value.value.(bool)
	if !ok {
		return false, NewJSONDeserializationError("Not a Boolean.", nil)
	}
	return result, nil
}

func JSONAsArray(value JSONRepresentation) ([]JSONRepresentation, error) {
	array, ok := value.value.([]any)
	if !ok {
		return nil, NewJSONDeserializationError("Not an array.", nil)
	}
	result := make([]JSONRepresentation, len(array))
	for index, item := range array {
		result[index] = NewJSONRepresentation(item)
	}
	return result, nil
}

// UnmarshalITFTraceRepresentation decodes an ITF trace assembled through the
// generic JSON adapter. It is the standard-library counterpart of Scala's
// JacksonRepresentation/ItfJsonToTla integration boundary.
func UnmarshalITFTraceRepresentation(value JSONRepresentation) ([]TraceState, error) {
	data, err := rawJSONRepresentation(value)
	if err != nil {
		return nil, err
	}
	return UnmarshalITFTrace(data)
}

// IRJSONEncoder exposes the node-level operations of the Apalache IR encoder.
type IRJSONEncoder struct{ Options IRJSONOptions }

func (encoder IRJSONEncoder) Expression(expression Expr) (JSONRepresentation, error) {
	value, err := encodeIRExpression(expression, encoder.Options)
	return NewJSONRepresentation(value), err
}

func (encoder IRJSONEncoder) Declaration(declaration Decl) (JSONRepresentation, error) {
	value, err := encodeIRDeclaration(declaration, encoder.Options)
	return NewJSONRepresentation(value), err
}

func (encoder IRJSONEncoder) Module(module *Module) (JSONRepresentation, error) {
	value, err := encodeIRModule(module, encoder.Options)
	if err != nil {
		return JSONRepresentation{}, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return JSONRepresentation{}, err
	}
	return ParseJSONRepresentation(data)
}

func (encoder IRJSONEncoder) Root(modules []*Module) (JSONRepresentation, error) {
	data, err := MarshalIRJSON(modules, encoder.Options)
	if err != nil {
		return JSONRepresentation{}, err
	}
	return ParseJSONRepresentation(data)
}

// IRJSONDecoder exposes the node-level operations of the Apalache IR decoder.
type IRJSONDecoder struct{ SourceStore *SourceStore }

func rawJSONRepresentation(value JSONRepresentation) (json.RawMessage, error) {
	data, err := json.Marshal(value.value)
	if err != nil {
		return nil, fmt.Errorf("encode JSON representation: %w", err)
	}
	return data, nil
}

func (decoder IRJSONDecoder) Expression(value JSONRepresentation) (Expr, error) {
	raw, err := rawJSONRepresentation(value)
	if err != nil {
		return nil, err
	}
	return decodeIRExpression(raw, decoder.SourceStore)
}

func (decoder IRJSONDecoder) Declaration(value JSONRepresentation) (Decl, error) {
	raw, err := rawJSONRepresentation(value)
	if err != nil {
		return nil, err
	}
	return decodeIRDeclaration(raw, decoder.SourceStore)
}

func (decoder IRJSONDecoder) Module(value JSONRepresentation) (*Module, error) {
	raw, err := rawJSONRepresentation(value)
	if err != nil {
		return nil, err
	}
	return decodeIRModule(raw, decoder.SourceStore)
}

func (decoder IRJSONDecoder) Root(value JSONRepresentation) ([]*Module, error) {
	data, err := rawJSONRepresentation(value)
	if err != nil {
		return nil, err
	}
	return UnmarshalIRJSON(data, decoder.SourceStore)
}

func (decoder IRJSONDecoder) SingleModule(value JSONRepresentation) (*Module, error) {
	data, err := rawJSONRepresentation(value)
	if err != nil {
		return nil, err
	}
	return UnmarshalSingleModuleIRJSON(data, decoder.SourceStore)
}
