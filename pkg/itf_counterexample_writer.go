package apalache

import "encoding/json"

// MarshalITFCounterexample writes the indented Informal Trace Format used by
// Scala's ItfCounterexampleWriter.
func MarshalITFCounterexample[T any](trace Trace[T], options ITFOptions) ([]byte, error) {
	compact, err := MarshalTraceITF(trace, options)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(compact, &value); err != nil {
		return nil, err
	}
	return json.MarshalIndent(value, "", "  ")
}

// EncodeITFCounterexampleState produces one indexed state object.
func EncodeITFCounterexampleState(state TraceState, index int) (map[string]any, error) {
	return encodeITFState(cloneTraceState(state), index)
}

// EncodeITFCounterexampleExpression converts one expression to its ITF JSON
// value representation.
func EncodeITFCounterexampleExpression(expression Expr) (any, error) {
	return EncodeITFValue(expression)
}
