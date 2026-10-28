package apalache

import (
	"fmt"
	"strings"
	"testing"
)

func TestExpressionConstructionAndRetagging(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	args := []Expr{x, one}
	equal, err := NewOperExpr(Op("EQ"), Typed(BoolT1), args...)
	if err != nil {
		t.Fatal(err)
	}
	args[0] = one
	if equal.Arguments()[0] != x {
		t.Fatal("operator expression retained caller's argument slice")
	}
	gotArgs := equal.Arguments()
	gotArgs[0] = one
	if equal.Arguments()[0] != x {
		t.Fatal("operator expression exposed its argument slice")
	}

	retagged := equal.WithTag(Untyped())
	if equal.ID() == retagged.ID() || !EqualExpr(equal, retagged) || EqualTypedExpr(equal, retagged) {
		t.Fatal("retagging did not make a structurally equal expression with a fresh ID and distinct tag")
	}
	if NullExpr().WithTag(Typed(IntT1)) != NullExpr() || NullExpr().Tag().IsTyped() {
		t.Fatal("null expression must be an always-untyped singleton")
	}
}

func TestOperatorExpressionInvariants(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	if _, err := NewOperExpr(Op("EQ"), Untyped(), x); err == nil || !strings.Contains(err.Error(), "unexpected arity") {
		t.Fatalf("wrong-arity error = %v", err)
	}
	if _, err := NewOperExpr(Op("Sequences!SelectSeq"), Untyped(), x, x); err == nil || !strings.Contains(err.Error(), "cannot be constructed") {
		t.Fatalf("prohibited-operator error = %v", err)
	}
	label := NewValueExpr(StringValue("label"), Untyped())
	argument := NewValueExpr(StringValue("arg"), Untyped())
	if _, err := NewOperExpr(Op("LABEL"), Untyped(), x, label, argument); err != nil {
		t.Fatalf("valid label: %v", err)
	}
	if _, err := NewOperExpr(Op("LABEL"), Untyped(), x, NewNameExpr("not-a-string-literal", Untyped())); err == nil {
		t.Fatal("LABEL accepted a non-string label")
	}
}

func TestRawSetOperatorsAndDeclarationApplicationsMatchScala(t *testing.T) {
	one := NewValueExpr(Int64Value(1), Untyped())
	two := NewValueExpr(Int64Value(2), Untyped())
	text := NewValueExpr(StringValue("hello"), Untyped())
	x := MustOperExpr(Op("SET_ENUM"), Untyped(), one, two, text)
	y := MustOperExpr(Op("SET_ENUM"), Untyped(), NewValueExpr(Int64Value(4), Untyped()))
	i := NewNameExpr("i", Untyped())
	for _, test := range []struct {
		operator string
		args     []Expr
	}{
		{"SET_UNION2", []Expr{x, y}}, {"SET_INTERSECT", []Expr{x, y}},
		{"SET_IN", []Expr{x, y}}, {"SET_NOT_IN", []Expr{x, y}}, {"SET_MINUS", []Expr{x, y}},
		{"SET_SUBSET_EQ", []Expr{x, y}}, {"SET_POWERSET", []Expr{y}}, {"SET_UNARY_UNION", []Expr{x}},
		{"SET_FILTER", []Expr{i, x, MustOperExpr(Op("SET_IN"), Untyped(), i, y)}},
		{"SET_MAP", []Expr{MustOperExpr(Op("SET_UNION2"), Untyped(), i, y), i, x}},
	} {
		expression, err := NewOperExpr(Op(test.operator), Untyped(), test.args...)
		if err != nil {
			t.Fatalf("%s: %v", test.operator, err)
		}
		if expression.Operator().Name() != test.operator || len(expression.Arguments()) != len(test.args) {
			t.Fatalf("%s shape was not preserved", test.operator)
		}
	}
	if empty, err := NewOperExpr(Op("SET_ENUM"), Untyped()); err != nil || len(empty.Arguments()) != 0 {
		t.Fatalf("empty set = %v, %v", empty, err)
	}
	if mixed, err := NewOperExpr(Op("SET_UNION2"), Untyped(), one, y); err != nil || mixed.Arguments()[0] != one {
		t.Fatalf("raw constructor enforced operand types: %v, %v", mixed, err)
	}

	for arity := 0; arity <= 3; arity++ {
		parameters := make([]OperatorParam, arity)
		arguments := make([]Expr, arity)
		for index := range parameters {
			parameters[index] = OperatorParam{Name: fmt.Sprintf("p%d", index), Arity: index}
			arguments[index] = NewNameExpr(fmt.Sprintf("a%d", index), Untyped())
		}
		declaration := NewOperDecl("A", parameters, NewNameExpr("body", Untyped()), Untyped())
		application, err := ApplyDeclarationUntyped(declaration, arguments...)
		if err != nil {
			t.Fatal(err)
		}
		got := application.Arguments()
		if got[0].(*NameExpr).Name() != "A" || len(got) != arity+1 {
			t.Fatalf("arity %d application = %#v", arity, got)
		}
		if arity > 0 {
			// Operator-valued formals deliberately accept ordinary names here.
			if got[arity] != arguments[arity-1] {
				t.Fatal("unchecked higher-order argument was changed")
			}
		}
		if _, err = ApplyDeclarationUntyped(declaration, append(arguments, one)...); err == nil {
			t.Fatalf("declaration arity %d accepted %d arguments", arity, arity+1)
		}
	}
	if _, err := ApplyDeclarationUntyped(nil); err == nil {
		t.Fatal("nil declaration was accepted")
	}
}

func TestLetInAndDeclarationSemantics(t *testing.T) {
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	params := []OperatorParam{ValueParam("x")}
	decl := NewOperDecl("One", params, one, Typed(IntT1))
	params[0].Name = "changed"
	if decl.FormalParams()[0].Name != "x" {
		t.Fatal("operator declaration retained caller's parameter slice")
	}
	decl.SetRecursive(true)
	copy := decl.WithTag(Untyped()).(*OperDecl)
	if !copy.IsRecursive() || copy.ID() == decl.ID() || !EqualDecl(decl, copy) || EqualTypedDecl(decl, copy) {
		t.Fatal("operator declaration copy semantics differ from Scala")
	}

	letDecls := []*OperDecl{decl}
	let := NewLetInExpr(NewNameExpr("One", Untyped()), Untyped(), letDecls...)
	letDecls[0] = copy
	if let.Declarations()[0] != decl {
		t.Fatal("LET-IN retained caller's declaration slice")
	}
	if !EqualExpr(let, let.WithTag(Typed(IntT1))) {
		t.Fatal("expression structural equality unexpectedly includes its type tag")
	}
}

func TestAssumptionNamesAndModuleViews(t *testing.T) {
	body := MustOperExpr(Op("EQ"), Typed(BoolT1),
		NewNameExpr("x", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	unnamed := NewUnnamedAssumeDecl(body, Typed(BoolT1))
	if unnamed.Name() != "ASSUME"+body.ID().String() {
		t.Fatalf("unnamed assumption name = %q", unnamed.Name())
	}
	named := NewNamedAssumeDecl("InitIsOne", body, Typed(BoolT1))
	if name, ok := named.DefinedName(); !ok || name != "InitIsOne" || named.Name() != name {
		t.Fatalf("named assumption = %q, %v", name, ok)
	}

	constant := NewConstDecl("N", Typed(IntT1))
	variable := NewVarDecl("x", Typed(IntT1))
	operator := NewOperDecl("Init", nil, body, Typed(BoolT1))
	theorem := NewTheoremDecl("Safety", body, Typed(BoolT1))
	input := []Decl{constant, variable, operator, unnamed, theorem}
	module := NewModule("Counter", input)
	input[0] = theorem
	if len(module.ConstDeclarations()) != 1 || len(module.VarDeclarations()) != 1 || len(module.OperDeclarations()) != 1 || len(module.AssumeDeclarations()) != 1 {
		t.Fatal("module declaration views do not match declaration kinds")
	}
	if module.Declarations()[0] != constant {
		t.Fatal("module retained caller's declaration slice")
	}
	module.AddProperty(PropertyTypeChecked)
	if !module.HasProperty(PropertyTypeChecked) || module.HasProperty(PropertyInlined) {
		t.Fatal("module properties not tracked independently")
	}
}

func TestStructuralEqualityIncludesChildrenButNotIdentity(t *testing.T) {
	left := MustOperExpr(Op("PLUS"), Untyped(),
		NewValueExpr(Int64Value(1), Untyped()), NewNameExpr("x", Untyped()))
	right := MustOperExpr(Op("PLUS"), Untyped(),
		NewValueExpr(Int64Value(1), Untyped()), NewNameExpr("x", Untyped()))
	different := MustOperExpr(Op("MINUS"), Untyped(),
		NewValueExpr(Int64Value(1), Untyped()), NewNameExpr("x", Untyped()))
	if left.ID() == right.ID() || !EqualExpr(left, right) || !EqualTypedExpr(left, right) {
		t.Fatal("equivalent expression trees are not structurally equal")
	}
	if EqualExpr(left, different) {
		t.Fatal("different operators compare equal")
	}
}

func TestInternalVariableMarkerSurvivesTypeAndDeepCopies(t *testing.T) {
	variable := NewVarDecl("aux", Typed(BoolT1))
	variable.MarkInternal()
	withTag := variable.WithTag(Typed(IntT1)).(*VarDecl)
	deep := NewDeepCopier(nil).Declaration(variable).(*VarDecl)
	if !variable.IsInternal() || !withTag.IsInternal() || !deep.IsInternal() {
		t.Fatalf("internal markers: original=%v withTag=%v deep=%v",
			variable.IsInternal(), withTag.IsInternal(), deep.IsInternal())
	}
}

func TestEveryDeclarationKindAndModuleViewMatchesScala(t *testing.T) {
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	two := NewValueExpr(Int64Value(2), Typed(IntT1))
	constant := NewConstDecl("C", Typed(IntT1))
	variable := NewVarDecl("x", Typed(IntT1))
	assumption := NewNamedAssumeDecl("Assumption", one, Typed(BoolT1))
	theorem := NewTheoremDecl("Theorem", one, Typed(BoolT1))
	operator := NewOperDecl("Op", []OperatorParam{ValueParam("p")}, one, Typed(IntT1))
	operator.SetRecursive(true)

	for _, declaration := range []Decl{constant, variable, assumption, theorem, operator} {
		copy := declaration.WithTag(Untyped())
		if copy.ID() == declaration.ID() || !EqualDecl(copy, declaration) || EqualTypedDecl(copy, declaration) {
			t.Errorf("%T retagging differs from Scala", declaration)
		}
	}
	operatorCopy := operator.WithTag(Untyped()).(*OperDecl)
	if !operatorCopy.IsRecursive() || operatorCopy.Body() != one || operatorCopy.FormalParams()[0] != ValueParam("p") {
		t.Fatal("operator copy lost recursion, body, or parameters")
	}
	operator.SetBody(two)
	if operator.Body() != two || operatorCopy.Body() != one {
		t.Fatal("mutable operator body aliased its copy")
	}

	constant2 := NewConstDecl("D", Untyped())
	variable2 := NewVarDecl("y", Untyped())
	operator2 := NewOperDecl("Other", nil, two, Untyped())
	assumption2 := NewUnnamedAssumeDecl(two, Untyped())
	module := NewModule("Ordered", []Decl{
		variable, constant, operator, assumption, theorem,
		constant2, variable2, operator2, assumption2,
	})
	if got := module.ConstDeclarations(); len(got) != 2 || got[0] != constant || got[1] != constant2 {
		t.Fatalf("constant view order = %#v", got)
	}
	if got := module.VarDeclarations(); len(got) != 2 || got[0] != variable || got[1] != variable2 {
		t.Fatalf("variable view order = %#v", got)
	}
	if got := module.OperDeclarations(); len(got) != 2 || got[0] != operator || got[1] != operator2 {
		t.Fatalf("operator view order = %#v", got)
	}
	if got := module.AssumeDeclarations(); len(got) != 2 || got[0] != assumption || got[1] != assumption2 {
		t.Fatalf("assumption view order = %#v", got)
	}
	declarations := module.Declarations()
	declarations[0] = constant
	if module.Declarations()[0] != variable {
		t.Fatal("module exposed its declaration slice")
	}
}
