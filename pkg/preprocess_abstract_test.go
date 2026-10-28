package apalache

import "testing"

func TestAbstractTransformerUsesFirstMatchingRuleAfterRecursion(t *testing.T) {
	one := intLiteral(1)
	two := intLiteral(2)
	plus := MustOperExpr(Op("PLUS"), Typed(IntT1), one, one)
	firstCalls, secondCalls := 0, 0
	transformer := NewAbstractTransformer(nil,
		func(expression Expr) (Expr, bool) {
			if _, literal := expression.(*ValueExpr); !literal {
				return nil, false
			}
			value, ok := constantInteger(expression)
			if !ok || value.Int64() != 1 {
				return nil, false
			}
			firstCalls++
			return two, true
		},
		func(expression Expr) (Expr, bool) {
			if _, literal := expression.(*ValueExpr); !literal {
				return nil, false
			}
			if _, ok := constantInteger(expression); !ok {
				return nil, false
			}
			secondCalls++
			return intLiteral(3), true
		},
	)
	got := transformer.Transform(plus).(*OperExpr)
	if firstCalls != 2 || secondCalls != 0 || !EqualTypedExpr(got.Arguments()[0], two) || !EqualTypedExpr(got.Arguments()[1], two) {
		t.Fatalf("ordered recursive rewrite = %v, calls = %d/%d", got, firstCalls, secondCalls)
	}
}

func TestAbstractTransformerLETBodiesAndIdentity(t *testing.T) {
	one := intLiteral(1)
	local := NewOperDecl("Local", nil, one, Typed(mustOperator(nil, IntT1)))
	let := NewLetInExpr(one, Typed(IntT1), local)
	letRuleCalled := false
	transformer := NewAbstractTransformer(nil,
		func(expression Expr) (Expr, bool) {
			if _, ok := expression.(*LetInExpr); ok {
				letRuleCalled = true
				return intLiteral(99), true
			}
			if _, literal := expression.(*ValueExpr); literal {
				value, ok := constantInteger(expression)
				if ok && value.Int64() == 1 {
					return intLiteral(2), true
				}
			}
			return nil, false
		},
	)
	got := transformer.Transform(let).(*LetInExpr)
	if letRuleCalled || got == let || !EqualTypedExpr(got.Body(), intLiteral(2)) ||
		!EqualTypedExpr(got.Declarations()[0].Body(), intLiteral(2)) {
		t.Fatalf("LET rewrite = %v, let rule called = %v", got, letRuleCalled)
	}

	leaf := NewNameExpr("x", Typed(IntT1))
	if unchanged := NewAbstractTransformer(nil).Transform(leaf); unchanged != leaf {
		t.Fatal("identity shallow transformation replaced an unchanged leaf")
	}
}

func TestAbstractTransformerVisitsLETBodyBeforeDefinitions(t *testing.T) {
	one := intLiteral(1)
	local := NewOperDecl("Local", nil, one, Typed(mustOperator(nil, IntT1)))
	let := NewLetInExpr(one, Typed(IntT1), local)
	visits := 0
	transformer := NewAbstractTransformer(nil, func(expression Expr) (Expr, bool) {
		if _, literal := expression.(*ValueExpr); !literal {
			return nil, false
		}
		visits++
		return intLiteral(int64(visits)), true
	})
	got := transformer.Transform(let).(*LetInExpr)
	if !EqualTypedExpr(got.Body(), intLiteral(1)) || !EqualTypedExpr(got.Declarations()[0].Body(), intLiteral(2)) {
		t.Fatalf("LET visit order body=%v definition=%v", got.Body(), got.Declarations()[0].Body())
	}
}

func TestAbstractTransformerTracksIntermediateOperatorOnlyForChangedArguments(t *testing.T) {
	one := intLiteral(1)
	plus := MustOperExpr(Op("PLUS"), Typed(IntT1), one, one)
	listener := &recordingListener{}
	transformer := NewAbstractTransformer(NewListenerTracker(listener), func(expression Expr) (Expr, bool) {
		if _, literal := expression.(*ValueExpr); literal {
			value, ok := constantInteger(expression)
			if ok && value.Int64() == 1 {
				return intLiteral(2), true
			}
		}
		return nil, false
	})
	got := transformer.Transform(plus)
	if got.ID() == plus.ID() {
		t.Fatal("changed arguments did not rebuild their parent operator")
	}
	parentToIntermediate, shallowIntermediate := 0, 0
	for _, event := range listener.expressions {
		if event[0] == plus && event[1] == got {
			parentToIntermediate++
		}
		if event[0] == got && event[1] == got {
			shallowIntermediate++
		}
	}
	if parentToIntermediate != 2 || shallowIntermediate != 1 {
		t.Fatalf("parent/intermediate tracking = %d/%d, events = %#v", parentToIntermediate, shallowIntermediate, listener.expressions)
	}

	listener = &recordingListener{}
	unchanged := NewAbstractTransformer(NewListenerTracker(listener)).Transform(plus)
	if unchanged != plus {
		t.Fatal("unchanged operator was rebuilt")
	}
	for _, event := range listener.expressions {
		if event[0] == plus && event[1] != plus {
			t.Fatalf("unchanged parent acquired an intermediate hold: %#v", event)
		}
	}
}
