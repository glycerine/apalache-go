package apalache

import "testing"

func TestNormalizeNNFBooleanConnectivesAndImplication(t *testing.T) {
	x := NewNameExpr("x", Typed(BoolT1))
	y := NewNameExpr("y", Typed(BoolT1))
	cases := []struct {
		input    Expr
		operator string
	}{
		{MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("AND"), Typed(BoolT1), x, y)), "OR"},
		{MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("OR"), Typed(BoolT1), x, y)), "AND"},
		{MustOperExpr(Op("IMPLIES"), Typed(BoolT1), x, y), "OR"},
		{MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("IMPLIES"), Typed(BoolT1), x, y)), "AND"},
	}
	for _, test := range cases {
		got, err := NormalizeNNF(test.input, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.(*OperExpr).Operator().Name() != test.operator {
			t.Errorf("NormalizeNNF(%v) = %v", test.input, got)
		}
	}
	double, err := NormalizeNNF(MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("NOT"), Typed(BoolT1), x)), nil)
	if err != nil || !EqualTypedExpr(double, x) {
		t.Fatalf("double negation = %v, %v", double, err)
	}
	truth, err := NormalizeNNF(MustOperExpr(Op("NOT"), Typed(BoolT1), NewValueExpr(BoolValue(false), Typed(BoolT1))), nil)
	if err != nil || !EqualTypedExpr(truth, NewValueExpr(BoolValue(true), Typed(BoolT1))) {
		t.Fatalf("negated false = %v, %v", truth, err)
	}
}

func TestNormalizeNNFEquivalenceAndEquality(t *testing.T) {
	x := NewNameExpr("x", Typed(BoolT1))
	y := NewNameExpr("y", Typed(BoolT1))
	for _, test := range []struct {
		input Expr
		want  string
	}{
		{MustOperExpr(Op("EQUIV"), Typed(BoolT1), x, y), "EQ"},
		{MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EQUIV"), Typed(BoolT1), x, y)), "NE"},
		{MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("NE"), Typed(BoolT1), x, y)), "EQ"},
	} {
		got, err := NormalizeNNF(test.input, nil)
		if err != nil || got.(*OperExpr).Operator().Name() != test.want {
			t.Errorf("normalization %v = %v, %v; want %s", test.input, got, err, test.want)
		}
	}
}

func TestNormalizeNNFQuantifiersRelationsAndMembership(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	set := NewNameExpr("S", Typed(mustSet(IntT1)))
	predicate := NewNameExpr("P", Typed(BoolT1))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x, set, predicate)
	got, err := NormalizeNNF(MustOperExpr(Op("NOT"), Typed(BoolT1), exists), nil)
	if err != nil {
		t.Fatal(err)
	}
	forall := got.(*OperExpr)
	if forall.Operator().Name() != "FORALL3" || forall.Arguments()[2].(*OperExpr).Operator().Name() != "NOT" {
		t.Fatalf("negated exists = %v", got)
	}
	for _, test := range []struct{ original, complement string }{{"LT", "GE"}, {"LE", "GT"}, {"GT", "LE"}, {"GE", "LT"}} {
		relation := MustOperExpr(Op(test.original), Typed(BoolT1), x, NewValueExpr(Int64Value(0), Typed(IntT1)))
		normalized, err := NormalizeNNF(MustOperExpr(Op("NOT"), Typed(BoolT1), relation), nil)
		if err != nil || normalized.(*OperExpr).Operator().Name() != test.complement {
			t.Errorf("%s complement = %v, %v", test.original, normalized, err)
		}
	}
	notin := MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1), x, set)
	normalized, err := NormalizeNNF(notin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.(*OperExpr).Operator().Name() != "NOT" || normalized.(*OperExpr).Arguments()[0].(*OperExpr).Operator().Name() != "SET_IN" {
		t.Fatalf("not-in = %v", normalized)
	}
}

func TestNormalizeNNFConditionalTemporalAndLabels(t *testing.T) {
	x := NewNameExpr("x", Typed(BoolT1))
	y := NewNameExpr("y", Typed(BoolT1))
	z := NewNameExpr("z", Typed(BoolT1))
	ite := MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), x, y, z)
	got, err := NormalizeNNF(MustOperExpr(Op("NOT"), Typed(BoolT1), ite), nil)
	if err != nil {
		t.Fatal(err)
	}
	args := got.(*OperExpr).Arguments()
	if args[0] != x || args[1].(*OperExpr).Operator().Name() != "NOT" || args[2].(*OperExpr).Operator().Name() != "NOT" {
		t.Fatalf("negated ITE = %v", got)
	}
	box := MustOperExpr(Op("GLOBALLY"), Typed(BoolT1), x)
	got, err = NormalizeNNF(MustOperExpr(Op("NOT"), Typed(BoolT1), box), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.(*OperExpr).Operator().Name() != "EVENTUALLY" || got.(*OperExpr).Arguments()[0].(*OperExpr).Operator().Name() != "NOT" {
		t.Fatalf("negated box = %v", got)
	}
	label := MustOperExpr(Op("LABEL"), Typed(BoolT1), MustOperExpr(Op("NOT"), Typed(BoolT1), NewValueExpr(BoolValue(false), Typed(BoolT1))), NewValueExpr(StringValue("L"), Typed(StrT1)))
	got, err = NormalizeNNF(label, nil)
	if err != nil || !EqualTypedExpr(got.(*OperExpr).Arguments()[0], NewValueExpr(BoolValue(true), Typed(BoolT1))) {
		t.Fatalf("label = %v, %v", got, err)
	}
	lead := MustOperExpr(Op("LEADS_TO"), Typed(BoolT1), x, y)
	got, err = NormalizeNNF(MustOperExpr(Op("NOT"), Typed(BoolT1), lead), nil)
	if err != nil || got.(*OperExpr).Operator().Name() != "NOT" || got.(*OperExpr).Arguments()[0] != lead {
		t.Fatalf("negated leads-to changed = %v, %v", got, err)
	}
	for _, operator := range []string{"GUARANTEES", "WEAK_FAIRNESS", "STRONG_FAIRNESS"} {
		expression := MustOperExpr(Op(operator), Typed(BoolT1), x, y)
		got, err = NormalizeNNF(MustOperExpr(Op("NOT"), Typed(BoolT1), expression), nil)
		if err != nil || got.(*OperExpr).Operator().Name() != "NOT" || got.(*OperExpr).Arguments()[0] != expression {
			t.Errorf("negated %s changed = %v, %v", operator, got, err)
		}
	}
}

func TestNormalizeNNFLetDefinitions(t *testing.T) {
	operatorType := mustOperator(nil, BoolT1)
	declaration := NewOperDecl("X", nil, MustOperExpr(Op("NOT"), Typed(BoolT1), NewValueExpr(BoolValue(false), Typed(BoolT1))), Typed(operatorType))
	body := MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr("X", Typed(operatorType))))
	root := MustOperExpr(Op("NOT"), Typed(BoolT1), NewLetInExpr(body, Typed(BoolT1), declaration))
	got, err := NormalizeNNF(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	if len(let.Declarations()) != 1 || !EqualTypedExpr(let.Declarations()[0].Body(), NewValueExpr(BoolValue(true), Typed(BoolT1))) {
		t.Fatalf("normalized LET = %v", got)
	}
	if let.Body().(*OperExpr).Operator().Name() != "OPER_APP" {
		t.Fatalf("normalized LET body = %v", let.Body())
	}
}

func TestNormalizeNNFIntroducesNegatedLocalOperator(t *testing.T) {
	operatorType := mustOperator(nil, BoolT1)
	declaration := NewOperDecl("X", nil, NewNameExpr("P", Typed(BoolT1)), Typed(operatorType))
	call := MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr("X", Typed(operatorType)))
	root := MustOperExpr(Op("NOT"), Typed(BoolT1), NewLetInExpr(call, Typed(BoolT1), declaration))
	got, err := NormalizeNNF(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	if len(let.Declarations()) != 2 || let.Declarations()[1].Name() != "NegX$" {
		t.Fatalf("negated definitions = %#v", let.Declarations())
	}
	body := let.Body().(*OperExpr)
	if body.Operator().Name() != "OPER_APP" || body.Arguments()[0].(*NameExpr).Name() != "NegX$" {
		t.Fatalf("negated body = %v", let.Body())
	}
	negatedDefinitionBody := let.Declarations()[1].Body().(*OperExpr)
	if negatedDefinitionBody.Operator().Name() != "NOT" {
		t.Fatalf("negated definition body = %v", negatedDefinitionBody)
	}
}

func TestNormalizeNNFTracksNegatedLocalOperatorReplacement(t *testing.T) {
	operatorType := mustOperator(nil, BoolT1)
	declaration := NewOperDecl("X", nil, NewNameExpr("P", Typed(BoolT1)), Typed(operatorType))
	call := MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr("X", Typed(operatorType)))
	root := MustOperExpr(Op("NOT"), Typed(BoolT1), NewLetInExpr(call, Typed(BoolT1), declaration))
	changes := NewChangeListener()
	got, err := NormalizeNNF(root, NewListenerTracker(changes))
	if err != nil {
		t.Fatal(err)
	}
	replacement := got.(*LetInExpr).Body()
	if changes.TraceBack(replacement.ID()) != call.ID() {
		t.Fatalf("negated local replacement traces to %s, want %s", changes.TraceBack(replacement.ID()), call.ID())
	}
}

func TestNormalizeNNFCompletesScalaCaseMatrix(t *testing.T) {
	boolName := func(name string) Expr { return NewNameExpr(name, Typed(BoolT1)) }
	intName := func(name string) Expr { return NewNameExpr(name, Typed(IntT1)) }
	not := func(expression Expr) Expr { return MustOperExpr(Op("NOT"), Typed(BoolT1), expression) }
	x, y := boolName("x"), boolName("y")
	bound, set := intName("i"), NewNameExpr("S", Typed(mustSet(IntT1)))

	for _, test := range []struct {
		name string
		in   Expr
		want Expr
	}{
		{
			"negated eventually",
			not(MustOperExpr(Op("EVENTUALLY"), Typed(BoolT1), x)),
			MustOperExpr(Op("GLOBALLY"), Typed(BoolT1), not(x)),
		},
		{
			"negated universal",
			not(MustOperExpr(Op("FORALL3"), Typed(BoolT1), bound, set, y)),
			MustOperExpr(Op("EXISTS3"), Typed(BoolT1), bound, set, not(y)),
		},
		{
			"negated membership normalizes operand",
			not(MustOperExpr(Op("SET_IN"), Typed(BoolT1), not(temporalBool(false)), NewNameExpr("B", Typed(mustSet(BoolT1))))),
			not(MustOperExpr(Op("SET_IN"), Typed(BoolT1), temporalBool(true), NewNameExpr("B", Typed(mustSet(BoolT1))))),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeNNF(test.in, nil)
			if err != nil || !EqualTypedExpr(got, test.want) {
				t.Fatalf("normalization = %v, want %v, error = %v", got, test.want, err)
			}
		})
	}

	boolSet := NewNameExpr("B", Typed(mustSet(BoolT1)))
	intZero := intLiteral(0)
	unchanged := []Expr{
		MustOperExpr(Op("LEADS_TO"), Typed(BoolT1), x, y),
		not(MustOperExpr(Op("LEADS_TO"), Typed(BoolT1), x, y)),
		MustOperExpr(Op("GUARANTEES"), Typed(BoolT1), x, y),
		not(MustOperExpr(Op("GUARANTEES"), Typed(BoolT1), x, y)),
		MustOperExpr(Op("WEAK_FAIRNESS"), Typed(BoolT1), x, y),
		not(MustOperExpr(Op("WEAK_FAIRNESS"), Typed(BoolT1), x, y)),
		MustOperExpr(Op("STRONG_FAIRNESS"), Typed(BoolT1), x, y),
		not(MustOperExpr(Op("STRONG_FAIRNESS"), Typed(BoolT1), x, y)),
		MustOperExpr(Op("EXISTS3"), Typed(BoolT1), bound, set, y),
		MustOperExpr(Op("FORALL3"), Typed(BoolT1), bound, set, y),
		MustOperExpr(Op("LT"), Typed(BoolT1), intName("n"), intZero),
		MustOperExpr(Op("LE"), Typed(BoolT1), intName("n"), intZero),
		MustOperExpr(Op("GT"), Typed(BoolT1), intName("n"), intZero),
		MustOperExpr(Op("GE"), Typed(BoolT1), intName("n"), intZero),
		MustOperExpr(Op("EQ"), Typed(BoolT1), intName("n"), intZero),
		MustOperExpr(Op("SET_IN"), Typed(BoolT1), x, boolSet),
	}
	for index, expression := range unchanged {
		got, err := NormalizeNNF(expression, nil)
		if err != nil || !EqualTypedExpr(got, expression) {
			t.Errorf("unchanged case %d = %v, want %v, error = %v", index, got, expression, err)
		}
	}
}
