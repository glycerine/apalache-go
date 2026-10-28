package apalache

import (
	"errors"
	"math/big"
	"testing"
)

func TestLegacyBuilderUntypedPredefsMatchScala(t *testing.T) {
	builder := NewLegacyBuilder()
	huge, _ := new(big.Int).SetString("1000000015943534656464398536", 10)
	decimal, err := builder.Decimal("1.111132454253626474876842798573504607")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		block LegacyBuilderExpr
		want  Expr
	}{
		{builder.Name("a"), NewNameExpr("a", Untyped())},
		{builder.BigInt(huge), NewValueExpr(NewIntValue(huge), Untyped())},
		{decimal, NewValueExpr(mustDecimal(t, "1.111132454253626474876842798573504607"), Untyped())},
		{builder.Bool(false), NewValueExpr(BoolValue(false), Untyped())},
		{builder.String("value"), NewValueExpr(StringValue("value"), Untyped())},
		{builder.BooleanSet(), NewValueExpr(BoolSet, Untyped())},
		{builder.StringSet(), NewValueExpr(StringSet, Untyped())},
		{builder.IntSet(), NewValueExpr(IntSet, Untyped())},
		{builder.NatSet(), NewValueExpr(NatSet, Untyped())},
	}
	for _, test := range tests {
		got, buildErr := BuildLegacyUntyped(test.block)
		if buildErr != nil || !EqualTypedExpr(got, test.want) {
			t.Errorf("BuildLegacyUntyped(%T) = %v, %v", test.block, got, buildErr)
		}
	}

	left, right := builder.Name("a"), builder.Int64(2)
	operation := builder.Operator(Op("PLUS"), left, right)
	got, err := BuildLegacyUntyped(operation)
	want := MustOperExpr(Op("PLUS"), Untyped(), NewNameExpr("a", Untyped()), NewValueExpr(Int64Value(2), Untyped()))
	if err != nil || !EqualTypedExpr(got, want) {
		t.Fatalf("legacy operation = %v, %v", got, err)
	}
	if _, err = BuildLegacyUntyped(builder.Operator(Op("PLUS"), left)); err == nil {
		t.Fatal("legacy operator arity was not checked at compilation")
	}

	alias, err := WithLegacyAlias(left, "A")
	if err != nil {
		t.Fatal(err)
	}
	if got, err = BuildLegacyUntyped(alias); err != nil || !EqualExpr(got, NewNameExpr("a", Untyped())) {
		t.Fatalf("untyped alias = %v, %v", got, err)
	}
	if _, err = WithLegacyAlias(alias, "B"); err == nil {
		t.Fatal("legacy alias overwrite was accepted")
	}
}

func mustDecimal(t *testing.T, text string) DecimalValue {
	t.Helper()
	value, err := NewDecimalValue(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestLegacyBuilderDeclarationsAndLetMatchScala(t *testing.T) {
	builder := NewLegacyBuilder()
	block := builder.Declaration("A", builder.Name("x"), ValueParam("x"))
	declaration, err := BuildLegacyUntypedDeclaration(block)
	if err != nil || declaration.Name() != "A" || len(declaration.FormalParams()) != 1 ||
		!EqualExpr(declaration.Body(), NewNameExpr("x", Untyped())) {
		t.Fatalf("legacy declaration = %#v, %v", declaration, err)
	}
	application, err := builder.ApplyDeclaration(declaration, builder.Name("value"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := BuildLegacyUntyped(application)
	want := MustOperExpr(Op("OPER_APP"), Untyped(), NewNameExpr("A", Untyped()), NewNameExpr("value", Untyped()))
	if err != nil || !EqualExpr(got, want) {
		t.Fatalf("legacy declaration application = %v, %v", got, err)
	}
	if _, err = builder.ApplyDeclaration(declaration); err == nil {
		t.Fatal("legacy declaration application accepted wrong arity")
	}

	let := builder.LetIn(builder.FromExpr(NewNameExpr("A", Untyped())), declaration)
	letExpr, err := BuildLegacyUntyped(let)
	if err != nil {
		t.Fatal(err)
	}
	actualLet := letExpr.(*LetInExpr)
	if actualLet.Body().(*NameExpr).Name() != "A" || actualLet.Declarations()[0] != declaration || actualLet.Tag().IsTyped() {
		t.Fatalf("legacy LET = %#v", actualLet)
	}
}

func TestLegacyTypedPredefsMatchScala(t *testing.T) {
	builder := NewLegacyBuilder()
	xAlias, _ := WithLegacyAlias(builder.Name("x"), "I")
	yAlias, _ := WithLegacyAlias(builder.Name("y"), "I")
	plus := builder.Operator(Op("PLUS"), xAlias, yAlias)
	got, err := BuildLegacyTyped(plus, map[string]Type{"I": IntT1}, IntT1)
	if err != nil {
		t.Fatal(err)
	}
	want := MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)), NewNameExpr("y", Typed(IntT1)))
	if !EqualTypedExpr(got, want) {
		t.Fatalf("legacy typed operation = %#v", got)
	}
	if _, err = BuildLegacyTyped(plus, map[string]Type{"I": IntT1}, nil); err == nil {
		t.Fatal("legacy typed operator accepted a missing result type")
	}
	if _, err = BuildLegacyTyped(builder.Name("x"), nil, nil); err == nil {
		t.Fatal("legacy typed name accepted a missing type")
	}
	missing, _ := WithLegacyAlias(builder.Name("x"), "missing")
	if _, err = BuildLegacyTyped(missing, nil, nil); err == nil {
		t.Fatal("legacy typed alias accepted a missing mapping")
	}
	placeholder, _ := WithLegacyAlias(builder.Name("x"), "?")
	if _, err = BuildLegacyTyped(placeholder, nil, nil); err == nil {
		t.Fatal("legacy typed alias accepted the placeholder mapping")
	}

	for _, test := range []struct {
		block LegacyBuilderExpr
		typ   Type
	}{
		{builder.Int64(1), IntT1}, {builder.Bool(true), BoolT1}, {builder.String("s"), StrT1},
		{builder.IntSet(), mustSetType(t, IntT1)}, {builder.NatSet(), mustSetType(t, IntT1)},
		{builder.BooleanSet(), mustSetType(t, BoolT1)}, {builder.StringSet(), mustSetType(t, StrT1)},
	} {
		expression, buildErr := BuildLegacyTyped(test.block, nil, nil)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		requireExprType(t, expression, test.typ)
	}
	modelType, _ := NewConstType("MODEL")
	model, err := BuildLegacyTyped(builder.String("value_OF_MODEL"), nil, modelType)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, model, modelType)
	decimal, _ := builder.Decimal("1.5")
	if _, err = BuildLegacyTyped(decimal, nil, nil); err == nil {
		t.Fatal("legacy TypedPredefs accepted Scala's unsupported decimal case")
	}
}

func TestLegacyTypedDeclarationsLetAndWrappedExpressionsMatchScala(t *testing.T) {
	builder := NewLegacyBuilder()
	operatorType, _ := NewOperatorType(nil, IntT1)
	block := builder.Declaration("A", builder.Int64(1))
	declaration, err := BuildLegacyTypedDeclaration(block, nil, operatorType)
	if err != nil {
		t.Fatal(err)
	}
	declarationType, ok := declaration.Tag().Type()
	if !ok || !EqualTypes(declarationType, operatorType) {
		t.Fatalf("legacy declaration type = %v", declaration.Tag())
	}
	requireExprType(t, declaration.Body(), IntT1)

	body := NewNameExpr("A", Typed(operatorType))
	let, err := BuildLegacyTyped(builder.LetIn(builder.FromExpr(body), declaration), nil, operatorType)
	if err != nil {
		t.Fatal(err)
	}
	letExpr := let.(*LetInExpr)
	requireExprType(t, letExpr, operatorType)
	if letExpr.Body() != body || letExpr.Declarations()[0] != declaration {
		t.Fatalf("typed legacy LET = %#v", letExpr)
	}
	if _, err = BuildLegacyTyped(builder.LetIn(builder.FromExpr(body), declaration), nil, nil); err == nil {
		t.Fatal("typed legacy LET accepted a missing top type")
	}

	original := NewNameExpr("x", Untyped())
	retagged, err := BuildLegacyTyped(builder.FromExpr(original), nil, BoolT1)
	if err != nil {
		t.Fatal(err)
	}
	if retagged.ID() == original.ID() || retagged.(*NameExpr).Name() != "x" {
		t.Fatal("wrapped expression retagging did not preserve Scala fresh-copy behavior")
	}
	requireExprType(t, retagged, BoolT1)
	unchanged, err := BuildLegacyTyped(builder.FromExpr(original), nil, nil)
	if err != nil || unchanged != original {
		t.Fatalf("wrapped expression without a top type = %v, %v", unchanged, err)
	}

	wrapped := WrapLegacyExpressions([]Expr{original, body})
	if len(wrapped) != 2 || wrapped[0].(LegacyWrappedExpr).Expr != original || wrapped[1].(LegacyWrappedExpr).Expr != body {
		t.Fatalf("wrapped expression sequence = %#v", wrapped)
	}
	aliased, err := BuildLegacyTypedAlias(builder.Int64(2), map[string]Type{"result": IntT1}, "result")
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, aliased, IntT1)
	aliasedDeclaration, err := BuildLegacyTypedDeclarationAlias(block, map[string]Type{"operator": operatorType}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if typ, _ := aliasedDeclaration.Tag().Type(); !EqualTypes(typ, operatorType) {
		t.Fatalf("alias-typed declaration = %v", aliasedDeclaration.Tag())
	}
	if _, err = BuildLegacyTypedDeclarationAlias(block, nil, "missing"); err == nil {
		t.Fatal("declaration alias lookup accepted a missing alias")
	}
	if LegacyTLA == nil || LegacyTLA == builder {
		t.Fatal("legacy convenience singleton is missing or aliases a fresh builder")
	}
}

func TestLegacyBuilderByNameMatchesScalaRegistry(t *testing.T) {
	builder := NewLegacyBuilder()
	for name, operator := range legacyBuilderOperatorRegistry() {
		count := -1
		for candidate := 0; candidate <= 12; candidate++ {
			if operator.IsCorrectArity(candidate) {
				count = candidate
				break
			}
		}
		if count < 0 {
			t.Fatalf("no tested arity for %s", name)
		}
		arguments := make([]LegacyBuilderExpr, count)
		for index := range arguments {
			arguments[index] = builder.FromExpr(NewNameExpr("x", Untyped()))
		}
		block, err := builder.ByName(name, arguments...)
		if err != nil {
			t.Errorf("ByName(%s): %v", name, err)
			continue
		}
		expression, err := BuildLegacyUntyped(block)
		if err != nil || expression.(*OperExpr).Operator().Name() != name {
			t.Errorf("ByName(%s) built %v, %v", name, expression, err)
		}
	}
	if _, err := builder.ByName("not an operator"); err == nil {
		t.Fatal("ByName accepted an unknown operator")
	}
	if got, err := BuildLegacyUntyped(builder.ByNameOrNull("not an operator")); err != nil || got != NullExpr() {
		t.Fatalf("ByNameOrNull unknown = %v, %v", got, err)
	}
	if got, err := BuildLegacyUntyped(builder.ByNameOrNull("PLUS", builder.Name("x"))); err != nil || got != NullExpr() {
		t.Fatalf("ByNameOrNull bad arity = %v, %v", got, err)
	}
	bad, _ := builder.ByName("PLUS", builder.Name("x"))
	if _, err := BuildLegacyUntyped(bad); err == nil {
		t.Fatal("ByName did not defer bad arity to compilation")
	}
	var builderError *BuilderError
	if _, err := builder.ByName("missing"); !errors.As(err, &builderError) {
		t.Fatalf("unknown operator error = %T, want BuilderError", err)
	}
}

func TestLegacyBuilderConvenienceSurfaceMatchesScala(t *testing.T) {
	b := NewLegacyBuilder()
	a, c, d := b.Name("a"), b.Name("c"), b.Name("d")
	tests := []struct {
		name     string
		block    LegacyBuilderExpr
		operator string
	}{
		{"eql", b.Eql(a, c), "EQ"}, {"neql", b.Neql(a, c), "NE"}, {"appOp", b.AppOp(a, c), "OPER_APP"},
		{"choose", b.Choose(a, c), "CHOOSE2"}, {"chooseIn", b.ChooseIn(a, c, d), "CHOOSE3"}, {"guess", b.Guess(a), "Apalache!Guess"}, {"label", b.Label(a, "L", "x"), "LABEL"},
		{"and", b.And(a, c), "AND"}, {"or", b.Or(a, c), "OR"}, {"not", b.Not(a), "NOT"}, {"impl", b.Impl(a, c), "IMPLIES"}, {"equiv", b.Equiv(a, c), "EQUIV"},
		{"forall", b.Forall(a, c), "FORALL2"}, {"forallIn", b.ForallIn(a, c, d), "FORALL3"}, {"exists", b.Exists(a, c), "EXISTS2"}, {"existsIn", b.ExistsIn(a, c, d), "EXISTS3"},
		{"prime", b.Prime(a), "PRIME"}, {"primeEq", b.PrimeEq(a, c), "EQ"}, {"stutt", b.Stutt(a, c), "STUTTER"}, {"nostutt", b.NoStutt(a, c), "NO_STUTTER"}, {"enabled", b.Enabled(a), "ENABLED"}, {"unchanged", b.Unchanged(a), "UNCHANGED"}, {"unchangedTuple", b.UnchangedTuple(a, c), "UNCHANGED"}, {"comp", b.Comp(a, c), "COMPOSE"},
		{"caseAny", b.CaseAny(a, c, d), "CASE_OTHER"}, {"caseSplit", b.CaseSplit(a, c), "CASE"}, {"caseOther", b.CaseOther(a, c, d), "CASE_OTHER"}, {"ite", b.Ite(a, c, d), "IF_THEN_ELSE"},
		{"AA", b.AA(a, c), "TEMPORAL_FORALL"}, {"EE", b.EE(a, c), "TEMPORAL_EXISTS"}, {"box", b.Box(a), "GLOBALLY"}, {"diamond", b.Diamond(a), "EVENTUALLY"}, {"guarantees", b.Guarantees(a, c), "GUARANTEES"}, {"leadsTo", b.LeadsTo(a, c), "LEADS_TO"}, {"SF", b.SF(a, c), "STRONG_FAIRNESS"}, {"WF", b.WF(a, c), "WEAK_FAIRNESS"},
		{"plus", b.Plus(a, c), "PLUS"}, {"minus", b.Minus(a, c), "MINUS"}, {"uminus", b.UMinus(a), "UNARY_MINUS"}, {"mult", b.Mult(a, c), "MULT"}, {"div", b.Div(a, c), "DIV"}, {"mod", b.Mod(a, c), "MOD"}, {"realDiv", b.RealDiv(a, c), "REAL_DIV"}, {"exp", b.Exp(a, c), "POW"}, {"dotdot", b.DotDot(a, c), "INT_RANGE"}, {"lt", b.Lt(a, c), "LT"}, {"gt", b.Gt(a, c), "GT"}, {"le", b.Le(a, c), "LE"}, {"ge", b.Ge(a, c), "GE"},
		{"card", b.Card(a), "FiniteSets!Cardinality"}, {"isFinite", b.IsFinite(a), "FiniteSets!IsFiniteSet"},
		{"appFun", b.AppFun(a, c), "FUN_APP"}, {"domain", b.Domain(a), "DOMAIN"}, {"record", b.Record(a, c), "RECORD"}, {"except", b.Except(a, c, d), "EXCEPT"}, {"funDef", b.FunDef(a, c, d), "FUN_CTOR"}, {"recFunDef", b.RecursiveFunDef(a, c, d), "FUN_REC_CTOR"}, {"recFunRef", b.RecursiveFunRef(), "FUN_REC_REF"}, {"tuple", b.Tuple(a, c), "TUPLE"},
		{"append", b.Append(a, c), "Sequences!Append"}, {"concat", b.Concat(a, c), "Sequences!Concat"}, {"head", b.Head(a), "Sequences!Head"}, {"tail", b.Tail(a), "Sequences!Tail"}, {"len", b.Len(a), "Sequences!Len"}, {"subseq", b.SubSeq(a, c, d), "Sequences!SubSeq"},
		{"enumSet", b.EnumSet(a), "SET_ENUM"}, {"emptySet", b.EmptySet(), "SET_ENUM"}, {"in", b.In(a, c), "SET_IN"}, {"notIn", b.NotIn(a, c), "SET_NOT_IN"}, {"cap", b.Cap(a, c), "SET_INTERSECT"}, {"cup", b.Cup(a, c), "SET_UNION2"}, {"union", b.Union(a), "SET_UNARY_UNION"}, {"filter", b.Filter(a, c, d), "SET_FILTER"}, {"map", b.Map(a, c, d), "SET_MAP"}, {"funSet", b.FunSet(a, c), "FUN_SET"}, {"recordSet", b.RecordSet(a, c), "RECORD_SET"}, {"seqSet", b.SeqSet(a), "Sequences!Seq"}, {"subsetEq", b.SubsetEq(a, c), "SET_SUBSET_EQ"}, {"setMinus", b.SetMinus(a, c), "SET_MINUS"}, {"times", b.Times(a, c), "SET_TIMES"}, {"powerSet", b.PowerSet(a), "SET_POWERSET"}, {"primeInSingleton", b.PrimeInSingleton(a, c), "SET_IN"},
		{"assign", b.Assign(a, c), "Apalache!:="}, {"assignPrime", b.AssignPrime(a, c), "Apalache!:="}, {"expand", b.Expand(a), "Apalache!Expand"}, {"setAsFun", b.SetAsFun(a), "Apalache!SetAsFun"}, {"mkSeq", b.MkSeq(a, c), "Apalache!MkSeq"}, {"skolem", b.Skolem(a), "Apalache!Skolem"}, {"constCard", b.ConstCardinality(a), "Apalache!ConstCardinality"},
		{"selectInSet", b.SelectInSet(a, c), "Apalache!SelectInSet"}, {"selectInFun", b.SelectInFun(a, c), "Apalache!SelectInFun"}, {"storeInSet", b.StoreInSet(a, c), "Apalache!StoreInSet"}, {"storeInFun", b.StoreInFun(a, c, d), "Apalache!StoreInSet"}, {"storeNotInSet", b.StoreNotInSet(a, c), "Apalache!storeNotInSet"}, {"storeNotInFun", b.StoreNotInFun(a, c), "Apalache!storeNotInFun"}, {"smtMap", b.SMTMap(Op("AND"), a, c), "Apalache!SmtMap_AND"}, {"unconstrain", b.UnconstrainArray(a), "Apalache!UnconstrainArray"},
		{"variant", b.Variant("A", a), "Variants!Variant"}, {"variantFilter", b.VariantFilter("A", a), "Variants!VariantFilter"}, {"variantTag", b.VariantTag(a), "Variants!VariantTag"}, {"variantUnsafe", b.VariantGetUnsafe("A", a), "Variants!VariantGetUnsafe"}, {"variantDefault", b.VariantGetOrElse("A", a, c), "Variants!VariantGetOrElse"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operation, ok := test.block.(LegacyOperatorExpr)
			if !ok || operation.Operator.Name() != test.operator {
				t.Fatalf("legacy shape = %T %#v, want operator %s", test.block, test.block, test.operator)
			}
		})
	}

	oddCase := b.CaseAny(a, c, d).(LegacyOperatorExpr)
	if oddCase.Operator.Name() != "CASE_OTHER" || len(oddCase.Arguments) != 3 || oddCase.Arguments[0] != a {
		t.Fatalf("odd CASE shape = %#v", oddCase)
	}
	evenCase := b.CaseAny(a, c, d, a).(LegacyOperatorExpr)
	if evenCase.Operator.Name() != "CASE" || len(evenCase.Arguments) != 4 {
		t.Fatalf("even CASE shape = %#v", evenCase)
	}
	label := b.Label(a, "L", "x", "y").(LegacyOperatorExpr)
	if len(label.Arguments) != 4 {
		t.Fatalf("label arguments = %v", label.Arguments)
	}
}
