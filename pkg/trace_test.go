package apalache

import (
	"bytes"
	"reflect"
	"testing"
)

func TestTraceOwnsStateModuleAndSortedLabels(t *testing.T) {
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1))})
	module.AddProperty(PropertyTypeChecked)
	states := []TraceState{{"x": intLiteral(1)}}
	labels := [][]string{{"z", "a", "z"}}
	trace := NewTrace(module, states, labels, "violation")

	states[0]["x"] = intLiteral(2)
	labels[0][0] = "changed"
	module.AddProperty(PropertyConfigured)
	if got := trace.States()[0]["x"].(*ValueExpr).Value().(IntValue).String(); got != "1" {
		t.Fatalf("trace state changed to %s", got)
	}
	if got := trace.Labels(); !reflect.DeepEqual(got, [][]string{{"a", "z"}}) {
		t.Fatalf("labels = %#v", got)
	}
	if !trace.Module().HasProperty(PropertyTypeChecked) || trace.Module().HasProperty(PropertyConfigured) {
		t.Fatal("trace module metadata was not captured independently")
	}
	returned := trace.States()
	returned[0]["x"] = intLiteral(3)
	if got := trace.States()[0]["x"].(*ValueExpr).Value().(IntValue).String(); got != "1" {
		t.Fatalf("returned state mutated trace to %s", got)
	}
	if trace.Data() != "violation" || trace.StateCount() != 1 || trace.LabelStateCount() != 1 {
		t.Fatalf("trace payload/counts = %q, %d, %d", trace.Data(), trace.StateCount(), trace.LabelStateCount())
	}
}

func TestMarshalTraceITFMatchesDirectCounterexampleEncoding(t *testing.T) {
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1))})
	states := []TraceState{{}, {"x": intLiteral(1)}}
	trace := NewTrace(module, states, [][]string{{"constant"}, {"next"}}, boolExpr(false))
	options := ITFOptions{Description: "trace"}
	got, err := MarshalTraceITF(trace, options)
	if err != nil {
		t.Fatal(err)
	}
	want, err := MarshalITFTrace(module, states, options)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("trace ITF = %s, want %s", got, want)
	}
}
