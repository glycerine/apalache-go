package apalache

import "testing"

func inlineApplication(name string, operatorType Type, resultType Type, arguments ...Expr) Expr {
	all := append([]Expr{NewNameExpr(name, Typed(operatorType))}, arguments...)
	return MustOperExpr(Op("OPER_APP"), Typed(resultType), all...)
}

func TestInlinerLocalAndTransitiveApplications(t *testing.T) {
	opType := mustOperator([]Type{IntT1}, IntT1)
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	two := NewValueExpr(Int64Value(2), Typed(IntT1))
	a := NewOperDecl("A", []OperatorParam{ValueParam("p")}, MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("p", Typed(IntT1)), one), Typed(opType))
	b := NewOperDecl("B", []OperatorParam{ValueParam("q")}, inlineApplication("A", opType, IntT1, NewNameExpr("q", Typed(IntT1))), Typed(opType))
	root := NewLetInExpr(inlineApplication("B", opType, IntT1, two), Typed(IntT1), a, b)
	got, err := NewInliner(nil, nil, true, nil).Transform(root)
	if err != nil {
		t.Fatal(err)
	}
	want := MustOperExpr(Op("PLUS"), Typed(IntT1), two, one)
	if !EqualTypedExpr(got, want) {
		t.Fatalf("inlined = %v, want %v", got, want)
	}
}

func TestInlinerNullaryRetentionModes(t *testing.T) {
	nullaryType := mustOperator(nil, IntT1)
	unaryType := mustOperator([]Type{IntT1}, IntT1)
	a := NewOperDecl("A", nil, NewValueExpr(Int64Value(1), Typed(IntT1)), Typed(nullaryType))
	b := NewOperDecl("B", []OperatorParam{ValueParam("x")}, NewValueExpr(Int64Value(2), Typed(IntT1)), Typed(unaryType))
	body := MustOperExpr(Op("PLUS"), Typed(IntT1), inlineApplication("A", nullaryType, IntT1), inlineApplication("B", unaryType, IntT1, NewValueExpr(Int64Value(0), Typed(IntT1))))
	root := NewLetInExpr(body, Typed(IntT1), a, b)
	kept, err := NewInliner(nil, nil, true, nil).Transform(root)
	if err != nil {
		t.Fatal(err)
	}
	let, ok := kept.(*LetInExpr)
	if !ok || len(let.Declarations()) != 1 || let.Declarations()[0].Name() != "A" {
		t.Fatalf("kept nullary = %v", kept)
	}
	removed, err := NewInliner(nil, nil, false, nil).Transform(root)
	if err != nil {
		t.Fatal(err)
	}
	want := MustOperExpr(Op("PLUS"), Typed(IntT1), NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	if !EqualTypedExpr(removed, want) {
		t.Fatalf("removed nullary = %v", removed)
	}
}

func TestInlinerPassByNameAndLambda(t *testing.T) {
	functionType := mustOperator([]Type{IntT1}, IntT1)
	callee := NewOperDecl("F", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(IntT1)), Typed(functionType))
	scopeLet := NewLetInExpr(NewNameExpr("F", Typed(functionType)), Typed(functionType), callee)
	kept, err := NewInliner(nil, nil, false, nil).Transform(scopeLet)
	if err != nil || !isPassByNameLet(kept.(*LetInExpr)) {
		t.Fatalf("pass-by-name = %v, %v", kept, err)
	}
	passedBody := MustOperExpr(Op("TUPLE"), Typed(mustTuple(functionType)), NewNameExpr("F", Typed(functionType)))
	passed := NewLetInExpr(passedBody, Typed(mustTuple(functionType)), callee)
	embedded, err := NewInliner(nil, nil, false, nil).Transform(passed)
	if err != nil {
		t.Fatal(err)
	}
	tuple := embedded.(*OperExpr)
	if local, ok := tuple.Arguments()[0].(*LetInExpr); !ok || !isPassByNameLet(local) {
		t.Fatalf("bare operator was not embedded: %v", embedded)
	}

	lambda := NewLetInExpr(NewNameExpr("LAMBDA", Typed(functionType)), Typed(functionType), NewOperDecl("LAMBDA", []OperatorParam{ValueParam("p")}, MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("p", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1))), Typed(functionType)))
	application := MustOperExpr(Op("OPER_APP"), Typed(IntT1), lambda, NewValueExpr(Int64Value(2), Typed(IntT1)))
	got, err := NewInliner(nil, nil, false, nil).Transform(application)
	if err != nil {
		t.Fatal(err)
	}
	want := MustOperExpr(Op("PLUS"), Typed(IntT1), NewValueExpr(Int64Value(2), Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	if !EqualTypedExpr(got, want) {
		t.Fatalf("lambda application = %v", got)
	}
}

func TestInlinerModuleAndNamedAssumptionScope(t *testing.T) {
	boolOperator := mustOperator(nil, BoolT1)
	assumption := NewNamedAssumeDecl("Ready", NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(BoolT1))
	use := NewOperDecl("Use", nil, inlineApplication("Ready", boolOperator, BoolT1), Typed(boolOperator))
	module := NewModule("M", []Decl{use, assumption})
	got, err := NewInliner(nil, nil, true, nil).Module(module)
	if err != nil {
		t.Fatal(err)
	}
	if !EqualTypedExpr(got.OperDeclarations()[0].Body(), NewValueExpr(BoolValue(true), Typed(BoolT1))) {
		t.Fatalf("named assumption not inlined: %v", got.OperDeclarations()[0].Body())
	}
}

func TestInlinerSpecializesPolymorphicBody(t *testing.T) {
	a, _ := NewVarType(0)
	calleeType := mustOperator([]Type{a}, mustSet(a))
	callee := NewOperDecl("Singleton", []OperatorParam{ValueParam("x")}, MustOperExpr(Op("SET_ENUM"), Typed(mustSet(a)), NewNameExpr("x", Typed(a))), Typed(calleeType))
	callType := mustOperator([]Type{IntT1}, mustSet(IntT1))
	root := NewLetInExpr(inlineApplication("Singleton", callType, mustSet(IntT1), NewValueExpr(Int64Value(1), Typed(IntT1))), Typed(mustSet(IntT1)), callee)
	got, err := NewInliner(nil, nil, true, nil).Transform(root)
	if err != nil {
		t.Fatal(err)
	}
	set := got.(*OperExpr)
	if typ, _ := set.Tag().Type(); !EqualTypes(typ, mustSet(IntT1)) {
		t.Fatalf("specialized set type = %v", typ)
	}
	if typ, _ := set.Arguments()[0].Tag().Type(); !EqualTypes(typ, IntT1) {
		t.Fatalf("specialized element type = %v", typ)
	}
}

func TestInstantiateEmbeddedOperatorSpecializesFromArguments(t *testing.T) {
	a, _ := NewVarType(0)
	setA := mustSet(a)
	operatorType := mustOperator([]Type{a}, setA)
	declaration := NewOperDecl("Embedded", []OperatorParam{ValueParam("x")},
		MustOperExpr(Op("SET_ENUM"), Typed(setA), NewNameExpr("x", Typed(a))), Typed(operatorType))
	embedded := NewLetInExpr(NewNameExpr("Embedded", Typed(operatorType)), Typed(operatorType), declaration)
	setStr := mustSet(StrT1)
	got, err := instantiateEmbeddedOperator(embedded, setStr, NewNameExpr("value", Typed(StrT1)))
	if err != nil {
		t.Fatal(err)
	}
	set, ok := got.(*OperExpr)
	if !ok || set.Operator().Name() != "SET_ENUM" {
		t.Fatalf("instantiated embedded operator = %v", got)
	}
	if typ, _ := set.Tag().Type(); !EqualTypes(typ, setStr) {
		t.Fatalf("instantiated result type = %v", typ)
	}
	if typ, _ := set.Arguments()[0].Tag().Type(); !EqualTypes(typ, StrT1) {
		t.Fatalf("instantiated element type = %v", typ)
	}
}

func TestInlinerTransitivePolymorphismIgnoresVariableNumbering(t *testing.T) {
	for _, numbers := range [][2]int{{0, 1}, {1, 0}, {0, 2}, {2, 0}} {
		a, _ := NewVarType(numbers[0])
		b, _ := NewVarType(numbers[1])
		pType := mustOperator([]Type{a}, mustSet(a))
		p := NewOperDecl("P", []OperatorParam{ValueParam("x")}, MustOperExpr(Op("SET_ENUM"), Typed(mustSet(a)), NewNameExpr("x", Typed(a))), Typed(pType))
		pAtB := mustOperator([]Type{b}, mustSet(b))
		qType := mustOperator([]Type{b}, mustSet(b))
		q := NewOperDecl("Q", []OperatorParam{ValueParam("y")}, inlineApplication("P", pAtB, mustSet(b), NewNameExpr("y", Typed(b))), Typed(qType))
		qAtInt := mustOperator([]Type{IntT1}, mustSet(IntT1))
		root := NewLetInExpr(inlineApplication("Q", qAtInt, mustSet(IntT1), NewValueExpr(Int64Value(7), Typed(IntT1))), Typed(mustSet(IntT1)), p, q)
		got, err := NewInliner(nil, nil, true, nil).Transform(root)
		if err != nil {
			t.Fatalf("variables %v: %v", numbers, err)
		}
		set, ok := got.(*OperExpr)
		if !ok || set.Operator().Name() != "SET_ENUM" {
			t.Fatalf("variables %v: result %v", numbers, got)
		}
		if typ, _ := set.Tag().Type(); !EqualTypes(typ, mustSet(IntT1)) {
			t.Fatalf("variables %v: set type %v", numbers, typ)
		}
		if typ, _ := set.Arguments()[0].Tag().Type(); !EqualTypes(typ, IntT1) {
			t.Fatalf("variables %v: element type %v", numbers, typ)
		}
	}
}

func TestInlinerDoesNotExpandRecursiveDeclaration(t *testing.T) {
	typ := mustOperator(nil, IntT1)
	recursive := NewOperDecl("R", nil, inlineApplication("R", typ, IntT1), Typed(typ))
	recursive.SetRecursive(true)
	root := NewLetInExpr(inlineApplication("R", typ, IntT1), Typed(IntT1), recursive)
	got, err := NewInliner(nil, nil, false, nil).Transform(root)
	if err != nil {
		t.Fatal(err)
	}
	application, ok := got.(*OperExpr)
	if !ok || application.Operator().Name() != "OPER_APP" || application.Arguments()[0].(*NameExpr).Name() != "R" {
		t.Fatalf("recursive call was expanded: %v", got)
	}
}
