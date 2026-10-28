package apalache

import (
	"errors"
	"testing"
)

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

func TestInstantiateEmbeddedOperatorPreservesNullaryLetSharing(t *testing.T) {
	boolOperator := mustOperator([]Type{IntT1}, BoolT1)
	nullaryInt := mustOperator(nil, IntT1)
	pick := NewOperDecl("Pick", nil, NewNameExpr("x", Typed(IntT1)), Typed(nullaryInt))
	pickCall := func() Expr {
		return MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr("Pick", Typed(nullaryInt)))
	}
	body := NewLetInExpr(MustOperExpr(Op("EQ"), Typed(BoolT1), pickCall(), pickCall()), Typed(BoolT1), pick)
	outer := NewOperDecl("Outer", []OperatorParam{ValueParam("x")}, body, Typed(boolOperator))
	embedded := NewLetInExpr(NewNameExpr("Outer", Typed(boolOperator)), Typed(boolOperator), outer)

	got, err := instantiateEmbeddedOperator(embedded, BoolT1, intLiteral(1))
	if err != nil {
		t.Fatal(err)
	}
	shared, ok := got.(*LetInExpr)
	if !ok || len(shared.Declarations()) != 1 {
		t.Fatalf("instantiated sharing = %v", got)
	}
	equality := shared.Body().(*OperExpr)
	left := equality.Arguments()[0].(*OperExpr).Arguments()[0].(*NameExpr)
	right := equality.Arguments()[1].(*OperExpr).Arguments()[0].(*NameExpr)
	if left.Name() != shared.Declarations()[0].Name() || right.Name() != left.Name() {
		t.Fatalf("shared calls = %s, %s; declaration = %s", left.Name(), right.Name(), shared.Declarations()[0].Name())
	}
}

func TestInlinerTransitivePolymorphismIgnoresVariableNumbering(t *testing.T) {
	for _, numbers := range [][2]int{{0, 1}, {1, 0}, {0, 2}, {2, 0}, {1, 2}, {2, 1}} {
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

func TestInlinerSpecializesTransitivePolymorphismAtModuleLevel(t *testing.T) {
	a, _ := NewVarType(2)
	b, _ := NewVarType(1)
	pType := mustOperator([]Type{a}, mustSet(a))
	p := NewOperDecl("P", []OperatorParam{ValueParam("x")},
		MustOperExpr(Op("SET_ENUM"), Typed(mustSet(a)), NewNameExpr("x", Typed(a))), Typed(pType))
	pAtB := mustOperator([]Type{b}, mustSet(b))
	qType := mustOperator([]Type{b}, mustSet(b))
	q := NewOperDecl("Q", []OperatorParam{ValueParam("y")},
		inlineApplication("P", pAtB, mustSet(b), NewNameExpr("y", Typed(b))), Typed(qType))
	qAtIntSet := mustOperator([]Type{mustSet(IntT1)}, mustSet(mustSet(IntT1)))
	x := NewOperDecl("X", nil,
		inlineApplication("Q", qAtIntSet, mustSet(mustSet(IntT1)), MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)))),
		Typed(mustOperator(nil, mustSet(mustSet(IntT1)))))
	got, err := NewInliner(nil, nil, true, nil).Module(NewModule("M", []Decl{x, q, p}))
	if err != nil {
		t.Fatal(err)
	}
	xBody := got.OperDeclarations()[2].Body().(*OperExpr)
	if typ, _ := xBody.Tag().Type(); !EqualTypes(typ, mustSet(mustSet(IntT1))) {
		t.Fatalf("module polymorphic result type = %v", typ)
	}
	if typ, _ := xBody.Arguments()[0].Tag().Type(); !EqualTypes(typ, mustSet(IntT1)) {
		t.Fatalf("module polymorphic element type = %v", typ)
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

func TestInlinerTracksLeavesApplicationsAndModuleDeclarations(t *testing.T) {
	operatorType := mustOperator([]Type{IntT1}, IntT1)
	parameter := NewNameExpr("x", Typed(IntT1))
	declaration := NewOperDecl("Id", []OperatorParam{ValueParam("x")}, parameter, Typed(operatorType))
	argument := NewValueExpr(Int64Value(1), Typed(IntT1))
	application := inlineApplication("Id", operatorType, IntT1, argument)
	root := NewLetInExpr(application, Typed(IntT1), declaration)
	listener := &recordingListener{}
	got, err := NewInliner(NewListenerTracker(listener), nil, true, nil).Transform(root)
	if err != nil || !EqualTypedExpr(got, argument) {
		t.Fatalf("tracked inline = %v, %v", got, err)
	}
	foundLeaf, foundApplication := false, false
	for _, event := range listener.expressions {
		foundLeaf = foundLeaf || event[0] == parameter && event[1] == parameter
		foundApplication = foundApplication || event[0] == application && EqualTypedExpr(event[1], argument)
	}
	if !foundLeaf || !foundApplication {
		t.Fatalf("missing tracking layers: leaf=%v application=%v events=%#v", foundLeaf, foundApplication, listener.expressions)
	}

	assumption := NewNamedAssumeDecl("Ready", NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(BoolT1))
	theorem := NewTheoremDecl("Safe", NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(BoolT1))
	listener = &recordingListener{}
	if _, err := NewInliner(NewListenerTracker(listener), nil, true, nil).Module(NewModule("M", []Decl{assumption, theorem})); err != nil {
		t.Fatal(err)
	}
	if len(listener.declarations) != 2 || listener.declarations[0][0] != assumption || listener.declarations[1][0] != theorem {
		t.Fatalf("module declaration events = %#v", listener.declarations)
	}
}

func TestInlinerReportsSignatureMismatchAsTypingException(t *testing.T) {
	calleeType := mustOperator([]Type{BoolT1}, BoolT1)
	callType := mustOperator([]Type{IntT1}, IntT1)
	declaration := NewOperDecl("F", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(BoolT1)), Typed(calleeType))
	root := NewLetInExpr(inlineApplication("F", callType, IntT1, intLiteral(1)), Typed(IntT1), declaration)
	listener := &recordingListener{}
	_, err := NewInliner(NewListenerTracker(listener), nil, true, nil).Transform(root)
	var typing *TypingException
	if !errors.As(err, &typing) || len(listener.declarations) != 1 || typing.CauseExprID() != listener.declarations[0][1].ID() {
		t.Fatalf("signature mismatch = %T %v", err, err)
	}
}

func TestInlinerHandlesNonbasicArgumentsAndModuleDependencyOrder(t *testing.T) {
	nullaryType := mustOperator(nil, IntT1)
	unaryType := mustOperator([]Type{IntT1}, IntT1)
	b := NewOperDecl("B", nil, intLiteral(42), Typed(nullaryType))
	a := NewOperDecl("A", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(IntT1)), Typed(unaryType))
	bCall := inlineApplication("B", nullaryType, IntT1)
	nestedCall := inlineApplication("A", unaryType, IntT1, bCall)
	local := NewLetInExpr(nestedCall, Typed(IntT1), b, a)
	got, err := NewInliner(nil, nil, false, nil).Transform(local)
	if err != nil || !EqualTypedExpr(got, intLiteral(42)) {
		t.Fatalf("nonbasic argument = %v, %v", got, err)
	}

	q := NewOperDecl("Q", []OperatorParam{ValueParam("y")},
		MustOperExpr(Op("PLUS"), Typed(IntT1), inlineApplication("P", unaryType, IntT1, NewNameExpr("y", Typed(IntT1))), intLiteral(1)), Typed(unaryType))
	p := NewOperDecl("P", []OperatorParam{ValueParam("x")},
		MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1)), Typed(unaryType))
	x := NewOperDecl("X", nil, inlineApplication("Q", unaryType, IntT1, intLiteral(1)), Typed(nullaryType))
	module := NewModule("M", []Decl{x, q, p})
	onlyX := KeepOnlyInlinedDeclarations(map[string]struct{}{"X": {}})
	inlined, err := NewInliner(nil, nil, true, onlyX).Module(module)
	if err != nil {
		t.Fatal(err)
	}
	if len(inlined.OperDeclarations()) != 1 || inlined.OperDeclarations()[0].Name() != "X" {
		t.Fatalf("module filter/dependency order = %#v", inlined.Declarations())
	}
	want := MustOperExpr(Op("PLUS"), Typed(IntT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(1)), intLiteral(1))
	if !EqualTypedExpr(inlined.OperDeclarations()[0].Body(), want) {
		t.Fatalf("transitive module inline = %v, want %v", inlined.OperDeclarations()[0].Body(), want)
	}
}

func TestInlinerAlwaysInlinesGlobalNullaryOperators(t *testing.T) {
	nullaryType := mustOperator(nil, IntT1)
	a := NewOperDecl("A", nil, intLiteral(1), Typed(nullaryType))
	x := NewOperDecl("X", nil, inlineApplication("A", nullaryType, IntT1), Typed(nullaryType))
	got, err := NewInliner(nil, nil, true, nil).Module(NewModule("M", []Decl{x, a}))
	if err != nil {
		t.Fatal(err)
	}
	if !EqualTypedExpr(got.OperDeclarations()[1].Body(), intLiteral(1)) {
		t.Fatalf("global nullary call = %v", got.OperDeclarations()[1].Body())
	}
}

func TestInlinerPreservesNestedPassByNameDefinitions(t *testing.T) {
	binaryType := mustOperator([]Type{IntT1, IntT1}, IntT1)
	f := NewOperDecl("F", []OperatorParam{ValueParam("old"), ValueParam("new")},
		NewNameExpr("new", Typed(IntT1)), Typed(binaryType))
	fold := MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1),
		NewNameExpr("F", Typed(binaryType)), intLiteral(0),
		MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1)))
	aBody := NewLetInExpr(fold, Typed(IntT1), f)
	a := NewOperDecl("A", []OperatorParam{ValueParam("old"), ValueParam("new")}, aBody, Typed(binaryType))
	invBody := MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1),
		NewNameExpr("A", Typed(binaryType)), intLiteral(0),
		MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1)))
	inv := NewOperDecl("Inv", nil, invBody, Typed(mustOperator(nil, IntT1)))
	got, err := NewInliner(nil, nil, true, nil).Module(NewModule("M", []Decl{a, inv}))
	if err != nil {
		t.Fatal(err)
	}
	outerFold := got.OperDeclarations()[1].Body().(*OperExpr)
	embeddedA, ok := outerFold.Arguments()[0].(*LetInExpr)
	if !ok || !isPassByNameLet(embeddedA) {
		t.Fatalf("outer pass-by-name = %v", outerFold.Arguments()[0])
	}
	innerFold := embeddedA.Declarations()[0].Body().(*OperExpr)
	embeddedF, ok := innerFold.Arguments()[0].(*LetInExpr)
	if !ok || !isPassByNameLet(embeddedF) {
		t.Fatalf("nested pass-by-name = %v", innerFold.Arguments()[0])
	}
}

func TestInlinerPassByNameKeepsCallSiteTypeVariables(t *testing.T) {
	a, _ := NewVarType(0)
	b, _ := NewVarType(1)
	calleeType := mustOperator([]Type{a}, a)
	callType := mustOperator([]Type{b}, b)
	declaration := NewOperDecl("F", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(a)), Typed(calleeType))
	root := NewLetInExpr(MustOperExpr(Op("TUPLE"), Typed(mustTuple(callType)), NewNameExpr("F", Typed(callType))),
		Typed(mustTuple(callType)), declaration)
	got, err := NewInliner(nil, nil, false, nil).Transform(root)
	if err != nil {
		t.Fatal(err)
	}
	embedded := got.(*OperExpr).Arguments()[0].(*LetInExpr)
	localType, _ := embedded.Declarations()[0].Tag().Type()
	if !EqualTypes(localType, callType) {
		t.Fatalf("embedded call-site type = %v, want %v", localType, callType)
	}
}
