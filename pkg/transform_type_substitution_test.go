package apalache

import "testing"

func TestTypeSubstitutorRewritesCompleteExpressionTree(t *testing.T) {
	a, _ := NewVarType(1)
	substitution, err := NewSubstitution(SubstitutionEntry{Class: NewEqClass(1), Type: IntT1})
	if err != nil {
		t.Fatal(err)
	}
	setA := mustSet(a)
	local := NewOperDecl("Local", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(a)), Typed(mustOperator([]Type{a}, a)))
	body := MustOperExpr(Op("SET_ENUM"), Typed(setA), NewNameExpr("v", Typed(a)))
	root := NewLetInExpr(body, Typed(setA), local)
	got, err := NewTypeSubstitutor(nil, substitution).Expression(root)
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	if typ, _ := let.Tag().Type(); !EqualTypes(typ, mustSet(IntT1)) {
		t.Fatalf("LET type = %v", typ)
	}
	if typ, _ := let.Body().(*OperExpr).Arguments()[0].Tag().Type(); !EqualTypes(typ, IntT1) {
		t.Fatalf("nested name type = %v", typ)
	}
	wantLocalType := mustOperator([]Type{IntT1}, IntT1)
	if typ, _ := let.Declarations()[0].Tag().Type(); !EqualTypes(typ, wantLocalType) {
		t.Fatalf("local type = %v", typ)
	}
	if root.ID() == got.ID() || local.ID() == let.Declarations()[0].ID() {
		t.Fatal("substitution reused IR identities")
	}
}

func TestTypeSubstitutorRewritesAllDeclarationKindsAndPreservesProperties(t *testing.T) {
	a, _ := NewVarType(2)
	substitution, _ := NewSubstitution(SubstitutionEntry{Class: NewEqClass(2), Type: BoolT1})
	value := NewNameExpr("p", Typed(a))
	module := NewModule("M", []Decl{
		NewConstDecl("C", Typed(a)), NewVarDecl("v", Typed(a)),
		NewNamedAssumeDecl("A", value, Typed(a)), NewTheoremDecl("T", value, Typed(a)),
	})
	module.AddProperty(PropertyTypeChecked)
	got, err := NewTypeSubstitutor(nil, substitution).Module(module)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyTypeChecked) {
		t.Fatal("module property lost")
	}
	for _, declaration := range got.Declarations() {
		if typ, ok := declaration.Tag().Type(); !ok || !EqualTypes(typ, BoolT1) {
			t.Fatalf("%s type = %v", declaration.Name(), typ)
		}
	}
	if typ, _ := got.AssumeDeclarations()[0].Body().Tag().Type(); !EqualTypes(typ, BoolT1) {
		t.Fatalf("assumption body type = %v", typ)
	}
}

func TestTypeSubstitutorLeavesUntypedTagsUntyped(t *testing.T) {
	substitutor := NewTypeSubstitutor(nil, EmptySubstitution())
	got, err := substitutor.Expression(NewNameExpr("x", Untyped()))
	if err != nil || got.Tag().IsTyped() {
		t.Fatalf("untyped substitution = %v, %v", got.Tag(), err)
	}
}

func TestTypeSubstitutorMatchesScalaScalarAndOperatorCases(t *testing.T) {
	a, _ := NewVarType(0)
	functionType := mustFun(IntT1, IntT1)
	substitution, err := NewSubstitution(SubstitutionEntry{Class: NewEqClass(0), Type: functionType})
	if err != nil {
		t.Fatal(err)
	}
	substitutor := NewTypeSubstitutor(nil, substitution)

	name := NewNameExpr("x", Typed(a))
	renamed, err := substitutor.Expression(name)
	if err != nil || renamed.ID() == name.ID() || !renamed.Tag().Equal(Typed(functionType)) {
		t.Fatalf("name substitution = %#v, %v", renamed, err)
	}
	for _, value := range []Expr{
		intLiteral(1),
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(StringValue("abc"), Typed(StrT1)),
	} {
		got, transformErr := substitutor.Expression(value)
		if transformErr != nil || got != value {
			t.Fatalf("concrete scalar %v was rebuilt: %#v, %v", value, got, transformErr)
		}
	}

	operatorType := mustOperator([]Type{a}, mustTuple(a, a))
	concreteOperatorType := mustOperator([]Type{functionType}, mustTuple(functionType, functionType))
	application := MustOperExpr(Op("OPER_APP"), Typed(mustTuple(a, a)),
		NewNameExpr("F", Typed(operatorType)), NewNameExpr("x", Typed(a)))
	got, err := substitutor.Expression(application)
	if err != nil {
		t.Fatal(err)
	}
	oper := got.(*OperExpr)
	arguments := oper.Arguments()
	if !oper.Tag().Equal(Typed(mustTuple(functionType, functionType))) ||
		!arguments[0].Tag().Equal(Typed(concreteOperatorType)) || !arguments[1].Tag().Equal(Typed(functionType)) {
		t.Fatalf("operator substitution tags = %s, %s, %s", oper.Tag(), arguments[0].Tag(), arguments[1].Tag())
	}
}

func TestTypeSubstitutorMatchesScalaLetInCase(t *testing.T) {
	a, _ := NewVarType(0)
	functionType := mustFun(IntT1, IntT1)
	pairType := mustTuple(a, a)
	operatorType := mustOperator([]Type{a}, pairType)
	parameter := NewNameExpr("x", Typed(a))
	pair := MustOperExpr(Op("TUPLE"), Typed(pairType), parameter, parameter)
	declaration := NewOperDecl("F", []OperatorParam{ValueParam("x")}, pair, Typed(operatorType))
	body := MustOperExpr(Op("OPER_APP"), Typed(pairType),
		NewNameExpr("F", Typed(operatorType)), NewNameExpr("x", Typed(a)))
	input := NewLetInExpr(body, Typed(pairType), declaration)
	substitution, _ := NewSubstitution(SubstitutionEntry{Class: NewEqClass(0), Type: functionType})
	got, err := NewTypeSubstitutor(nil, substitution).Expression(input)
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	concretePair := mustTuple(functionType, functionType)
	concreteOperator := mustOperator([]Type{functionType}, concretePair)
	if !let.Tag().Equal(Typed(concretePair)) || !let.Body().Tag().Equal(Typed(concretePair)) ||
		!let.Declarations()[0].Tag().Equal(Typed(concreteOperator)) ||
		!let.Declarations()[0].Body().Tag().Equal(Typed(concretePair)) {
		t.Fatalf("LET substitution = %#v", let)
	}
}

func TestTypeSubstitutorPreservesUnchangedOperatorIdentity(t *testing.T) {
	left, right := intLiteral(1), intLiteral(2)
	operator := MustOperExpr(Op("PLUS"), Typed(IntT1), left, right)
	got, err := NewTypeSubstitutor(nil, EmptySubstitution()).Expression(operator)
	if err != nil || got != operator || got.(*OperExpr).Arguments()[0] != left || got.(*OperExpr).Arguments()[1] != right {
		t.Fatalf("unchanged operator was rebuilt: %#v, %v", got, err)
	}
}
