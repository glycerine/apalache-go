package apalache

import (
	"strings"
	"testing"
)

func TestMarshalDecodedTraceFormatsRoundTrip(t *testing.T) {
	internal := NewVarDecl("__internal", Typed(BoolT1))
	internal.MarkInternal()
	module := NewModule("Root", []Decl{
		NewConstDecl("N", Typed(IntT1)),
		NewVarDecl("x", Typed(IntT1)),
		NewVarDecl("y", Typed(BoolT1)),
		internal,
	})
	states := []DecodedState{
		{Values: map[string]Expr{
			"N": intLiteral(10), "x": intLiteral(1), "y": boolExpr(false), "__internal": boolExpr(true),
		}},
		{Values: map[string]Expr{
			"N": intLiteral(10), "x": intLiteral(2), "y": boolExpr(true), "__internal": boolExpr(false),
		}},
	}
	data, err := MarshalDecodedTraceApalacheJSON(module, states, boolExpr(true))
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := UnmarshalApalacheJSONTrace(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundTrip) != 2 || roundTrip[0]["N"] != nil || roundTrip[0]["__internal"] != nil ||
		!EqualExpr(roundTrip[0]["x"], intLiteral(1)) || !EqualExpr(roundTrip[1]["y"], boolExpr(true)) {
		t.Fatalf("round-trip trace = %#v", roundTrip)
	}
	tla, err := FormatDecodedTraceTLA(module, states, boolExpr(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"MODULE counterexample", "EXTENDS Root", "ConstInit == N = 10",
		"State0 == x = 1 /\\ y = FALSE", "State1 == x = 2 /\\ y = TRUE",
		"InvariantViolation == TRUE",
	} {
		if !strings.Contains(tla, fragment) {
			t.Errorf("TLA trace missing %q:\n%s", fragment, tla)
		}
	}
	tlc, err := FormatDecodedTraceTLC(module, states)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"@!@!@STARTMSG 2110:1 @!@!@", "1: <Initial predicate>", "2: <Next>",
		"/\\ x = 1\n/\\ y = FALSE", "/\\ x = 2\n/\\ y = TRUE",
	} {
		if !strings.Contains(tlc, fragment) {
			t.Errorf("TLC trace missing %q:\n%s", fragment, tlc)
		}
	}
	if strings.Contains(tlc, "N = 10") || strings.Contains(tlc, "__internal") {
		t.Fatalf("TLC trace exposed constants/internal state:\n%s", tlc)
	}
}

func TestMarshalDecodedTraceFormatsRejectEmptyTrace(t *testing.T) {
	module := NewModule("Root", nil)
	if _, err := MarshalDecodedTraceApalacheJSON(module, nil, nil); err == nil {
		t.Fatal("legacy JSON accepted an empty decoded trace")
	}
	if _, err := FormatDecodedTraceTLA(module, nil, nil); err == nil {
		t.Fatal("TLA accepted an empty decoded trace")
	}
	if _, err := FormatDecodedTraceTLC(module, nil); err == nil {
		t.Fatal("TLC accepted an empty decoded trace")
	}
}

func TestUnmarshalApalacheJSONTrace(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(BoolT1))
	state := func(xValue int64, yValue bool) Expr {
		return MustOperExpr(Op("AND"), Typed(BoolT1),
			MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(xValue)),
			MustOperExpr(Op("EQ"), Typed(BoolT1), y, NewValueExpr(BoolValue(yValue), Typed(BoolT1))))
	}
	module := NewModule("LegacyTrace", []Decl{
		NewOperDecl("CInit", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("State0", nil, state(1, false), Typed(BoolT1)),
		NewOperDecl("State1", nil, state(2, true), Typed(BoolT1)),
		NewOperDecl("Inv", nil, boolExpr(true), Typed(BoolT1)),
	})
	data, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	states, err := UnmarshalApalacheJSONTrace(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("decoded %d states", len(states))
	}
	for index, want := range []struct {
		x int64
		y bool
	}{{1, false}, {2, true}} {
		if !EqualExpr(states[index]["x"], intLiteral(want.x)) ||
			!EqualExpr(states[index]["y"], NewValueExpr(BoolValue(want.y), Typed(BoolT1))) {
			t.Fatalf("state %d = %#v", index, states[index])
		}
	}
}

func TestUnmarshalApalacheJSONTraceRejectsMalformedState(t *testing.T) {
	module := NewModule("BadTrace", []Decl{
		NewOperDecl("CInit", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("State0", nil, boolExpr(false), Typed(BoolT1)),
		NewOperDecl("Inv", nil, boolExpr(true), Typed(BoolT1)),
	})
	data, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = UnmarshalApalacheJSONTrace(data); err == nil {
		t.Fatal("accepted a trace state without assignments")
	}
}

func TestUnmarshalApalacheJSONTraceAcceptsEmptyState(t *testing.T) {
	module := NewModule("EmptyTrace", []Decl{
		NewOperDecl("CInit", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("State0", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("Inv", nil, boolExpr(true), Typed(BoolT1)),
	})
	data, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	states, err := UnmarshalApalacheJSONTrace(data)
	if err != nil || len(states) != 1 || len(states[0]) != 0 {
		t.Fatalf("empty legacy state = %#v, %v", states, err)
	}
}
