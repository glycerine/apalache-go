package apalache

import "testing"

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
		NewOperDecl("State0", nil, boolExpr(true), Typed(BoolT1)),
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
