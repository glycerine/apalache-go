package apalache

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidateRemoteApalacheConfigAcceptsMemoryAndSafeOptions(t *testing.T) {
	source := NewStringInputSource("---- MODULE M ----\n====", nil, InputFormatTLA)
	tuning := map[string]string{"search.invariant.mode": "before"}
	length := 3
	config := ApalacheConfig{Source: source, Checker: CheckerPatch{Length: &length, Tuning: &tuning}}
	result := ValidateRemoteApalacheConfig(config)
	if !result.IsSuccess() || !reflect.DeepEqual(result.RequireValue(), config) {
		t.Fatalf("safe remote config = %#v", result.Errors())
	}
}

func TestParseRemoteApalacheConfigRejectsEveryFilesystemField(t *testing.T) {
	cases := []struct{ path, json string }{
		{"$.config-file", `{"config-file":"config.json"}`},
		{"$.out-dir", `{"out-dir":"out"}`},
		{"$.run-dir", `{"run-dir":"run"}`},
		{"$.output", `{"output":"output.tla"}`},
		{"$.source", `{"source":"input.tla"}`},
		{"$.checker.config", `{"checker":{"config":"M.cfg"}}`},
		{"$.tracee.trace", `{"tracee":{"trace":"trace.itf.json"}}`},
	}
	for _, test := range cases {
		result := ParseRemoteApalacheConfig(test.json)
		if result.IsSuccess() || len(result.Errors()) == 0 || !strings.HasPrefix(result.Errors()[0], test.path) {
			t.Errorf("%s => %#v", test.path, result.Errors())
		}
	}
}

func TestParseRemoteApalacheConfigRejectsAllFileWritingTuningKeysInSortedOrder(t *testing.T) {
	keys := RemoteFileWritingTuningKeys()
	if len(keys) != 12 {
		t.Fatalf("file-writing tuning keys = %#v", keys)
	}
	tuning := make(map[string]string, len(keys)+1)
	for _, key := range keys {
		tuning[key] = "true"
	}
	tuning["search.invariant.mode"] = "before"
	config := ApalacheConfig{Checker: CheckerPatch{Tuning: &tuning}}
	result := ValidateRemoteApalacheConfig(config)
	if result.IsSuccess() || len(result.Errors()) != len(keys) {
		t.Fatalf("unsafe tuning errors = %#v", result.Errors())
	}
	for index, key := range keys {
		if !strings.Contains(result.Errors()[index], key) {
			t.Fatalf("error %d = %q, want %q", index, result.Errors()[index], key)
		}
	}
	keys[0] = "mutated"
	if RemoteFileWritingTuningKeys()[0] == "mutated" {
		t.Fatal("tuning key list aliases internal policy")
	}
}

func TestParseRemoteApalacheConfigSourceAndLegacyShapes(t *testing.T) {
	result := ParseRemoteApalacheConfig(`{"source":{"kind":"string","content":"---- MODULE M ----\n====","aux":[],"format":"tla"}}`)
	if !result.IsSuccess() {
		t.Fatal(result.Errors())
	}
	if _, ok := result.RequireValue().Source.(StringInputSource); !ok {
		t.Fatalf("source = %#v", result.RequireValue().Source)
	}

	legacy := ParseRemoteApalacheConfig(`{"input":{"source":{"type":"string","content":"module M {}","format":"qnt"}},"checker":{"temporal-props":["q::temporalProps"]}}`)
	want := map[string]bool{
		"$.input: Unknown configuration key.":                  false,
		"$.checker.temporal-props: Unknown configuration key.": false,
	}
	for _, message := range legacy.Errors() {
		if _, found := want[message]; found {
			want[message] = true
		}
	}
	if legacy.IsSuccess() || !want["$.input: Unknown configuration key."] || !want["$.checker.temporal-props: Unknown configuration key."] {
		t.Fatalf("legacy errors = %#v", legacy.Errors())
	}
}
