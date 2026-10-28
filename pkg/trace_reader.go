package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type TraceJSONKind uint8

const (
	ITFTraceJSON TraceJSONKind = iota
	ApalacheTraceJSON
)

type TraceJSON struct {
	kind TraceJSONKind
	json []byte
}

func NewTraceJSON(kind TraceJSONKind, data []byte) (TraceJSON, error) {
	if !json.Valid(data) {
		return TraceJSON{}, NewJSONDeserializationError("unable to read source as JSON", nil)
	}
	return TraceJSON{kind: kind, json: append([]byte(nil), data...)}, nil
}

func (trace TraceJSON) Kind() TraceJSONKind { return trace.kind }
func (trace TraceJSON) JSON() []byte        { return append([]byte(nil), trace.json...) }

type UJSONTraceReader struct{}

func NewUJSONTraceReader() *UJSONTraceReader { return &UJSONTraceReader{} }

func (reader *UJSONTraceReader) Read(source InputSource) (TraceJSON, error) {
	if source == nil {
		return TraceJSON{}, fmt.Errorf("tried to load state from an invalid source: <nil>")
	}
	content, err := source.ReadUTF8()
	if err != nil {
		return TraceJSON{}, err
	}
	switch source.Format() {
	case InputFormatITF:
		return NewTraceJSON(ITFTraceJSON, []byte(content))
	case InputFormatJSON:
		return NewTraceJSON(ApalacheTraceJSON, []byte(content))
	default:
		return TraceJSON{}, fmt.Errorf("tried to load state from an invalid source: %s", source)
	}
}

func (reader *UJSONTraceReader) Convert(trace TraceJSON) ([]TraceState, error) {
	switch trace.kind {
	case ITFTraceJSON:
		return UnmarshalITFTrace(trace.json)
	case ApalacheTraceJSON:
		return UnmarshalApalacheJSONTrace(trace.json)
	default:
		return nil, fmt.Errorf("unexpected trace JSON kind %d", trace.kind)
	}
}

func (reader *UJSONTraceReader) GetTraceLength(trace TraceJSON) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(trace.json))
	var root map[string]json.RawMessage
	if err := decoder.Decode(&root); err != nil {
		return 0, NewJSONDeserializationError("unable to decode trace JSON", err)
	}
	switch trace.kind {
	case ITFTraceJSON:
		var states []json.RawMessage
		raw, found := root[ITFStatesField]
		if !found || json.Unmarshal(raw, &states) != nil {
			return 0, NewJSONDeserializationError("provided JSON does not comply with the ITF format", nil)
		}
		return len(states), nil
	case ApalacheTraceJSON:
		var modules []struct {
			Declarations []json.RawMessage `json:"declarations"`
		}
		raw, found := root["modules"]
		if !found || json.Unmarshal(raw, &modules) != nil || len(modules) == 0 || modules[0].Declarations == nil {
			return 0, NewJSONDeserializationError("provided JSON does not comply with the Apalache JSON format", nil)
		}
		return len(modules[0].Declarations) - 2, nil
	default:
		return 0, fmt.Errorf("unexpected trace JSON kind %d", trace.kind)
	}
}
