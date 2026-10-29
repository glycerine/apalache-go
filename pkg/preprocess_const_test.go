package apalache

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
)

func TestSimplifyConstantsArithmeticBooleanAndControl(t *testing.T) {
	i := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	b := func(value bool) Expr { return NewValueExpr(BoolValue(value), Typed(BoolT1)) }
	x := NewNameExpr("x", Typed(IntT1))
	p := NewNameExpr("p", Typed(BoolT1))
	cases := []struct {
		name        string
		input, want Expr
	}{
		{"nested arithmetic", MustOperExpr(Op("PLUS"), Typed(IntT1), i(7), MustOperExpr(Op("MINUS"), Typed(IntT1), i(4), i(1))), i(10)},
		{"neutral arithmetic", MustOperExpr(Op("MULT"), Typed(IntT1), i(1), x), x},
		{"same subtraction", MustOperExpr(Op("MINUS"), Typed(IntT1), x, x), i(0)},
		{"double negation", MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("NOT"), Typed(BoolT1), p)), p},
		{"conjunction", MustOperExpr(Op("AND"), Typed(BoolT1), b(true), p, b(true)), p},
		{"implication", MustOperExpr(Op("IMPLIES"), Typed(BoolT1), p, b(false)), MustOperExpr(Op("NOT"), Typed(BoolT1), p)},
		{"if predicate", MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), p, b(true), b(false)), p},
		{"if introduced double negation", MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), MustOperExpr(Op("NOT"), Typed(BoolT1), p), b(false), NewNameExpr("q", Typed(BoolT1))), MustOperExpr(Op("AND"), Typed(BoolT1), p, NewNameExpr("q", Typed(BoolT1)))},
		{"not equal", MustOperExpr(Op("NE"), Typed(BoolT1), i(3), i(3)), b(false)},
		{"relation", MustOperExpr(Op("LT"), Typed(BoolT1), i(-3), i(2)), b(true)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := SimplifyConstants(test.input, IdleTracker{})
			if err != nil || !EqualTypedExpr(got, test.want) {
				t.Fatalf("SimplifyConstants() = %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestSimplifyConstantsBigIntegersAndErrors(t *testing.T) {
	huge := new(big.Int).Exp(big.NewInt(10), big.NewInt(80), nil)
	left := NewValueExpr(NewIntValue(huge), Typed(IntT1))
	right := NewValueExpr(NewIntValue(huge), Typed(IntT1))
	got, err := SimplifyConstants(MustOperExpr(Op("PLUS"), Typed(IntT1), left, right), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := NewValueExpr(NewIntValue(new(big.Int).Mul(huge, big.NewInt(2))), Typed(IntT1))
	if !EqualTypedExpr(got, want) {
		t.Fatalf("big integer sum = %v, want %v", got, want)
	}
	zero := NewValueExpr(Int64Value(0), Typed(IntT1))
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	for _, test := range []struct{ operator, message string }{{"DIV", "Division by zero at"}, {"MOD", "Mod by zero at"}, {"POW", "0 ^ 0"}} {
		_, err := SimplifyConstants(MustOperExpr(Op(test.operator), Typed(IntT1), one, zero), nil)
		if test.operator == "POW" {
			_, err = SimplifyConstants(MustOperExpr(Op(test.operator), Typed(IntT1), zero, zero), nil)
		}
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatalf("%s error = %v", test.operator, err)
		}
		var inputError *TlaInputError
		if !errors.As(err, &inputError) {
			t.Fatalf("%s error type = %T, want *TlaInputError", test.operator, err)
		}
	}

	tooLargePower := new(big.Int).Add(big.NewInt(2147483647), big.NewInt(1000))
	_, err = SimplifyConstants(MustOperExpr(Op("POW"), Typed(IntT1),
		NewValueExpr(Int64Value(8888888), Typed(IntT1)),
		NewValueExpr(NewIntValue(tooLargePower), Typed(IntT1))), nil)
	if err == nil || err.Error() != "The power at 8888888 ^ 2147484647 exceedes the limit of 2147483647" {
		t.Fatalf("large power error = %v", err)
	}
	_, err = SimplifyConstants(MustOperExpr(Op("POW"), Typed(IntT1),
		NewValueExpr(Int64Value(2147483647), Typed(IntT1)),
		NewValueExpr(Int64Value(2103446789), Typed(IntT1))), nil)
	if err == nil || err.Error() != "The result of 2147483647 ^ 2103446789 exceedes the limit of 1.7976931348623157E308" {
		t.Fatalf("oversized result error = %v", err)
	}
	_, err = SimplifyConstants(MustOperExpr(Op("POW"), Typed(IntT1), one, NewValueExpr(Int64Value(-1), Typed(IntT1))), nil)
	if err == nil || err.Error() != "Negative power at 1 ^ (-1)" {
		t.Fatalf("negative power error = %v", err)
	}
}

func TestSimplifyConstantsLiteralSetsQuantifiersAndCardinality(t *testing.T) {
	i := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	b := func(value bool) Expr { return NewValueExpr(BoolValue(value), Typed(BoolT1)) }
	setType := mustSet(IntT1)
	set := func(values ...Expr) Expr { return MustOperExpr(Op("SET_ENUM"), Typed(setType), values...) }
	left, right := set(i(4), i(2), i(2)), set(i(3), i(2))
	union, err := SimplifyConstants(MustOperExpr(Op("SET_UNION2"), Typed(setType), left, right), nil)
	if err != nil || !EqualTypedExpr(union, set(i(2), i(3), i(4))) {
		t.Fatalf("union = %v, %v", union, err)
	}
	intersection, _ := SimplifyConstants(MustOperExpr(Op("SET_INTERSECT"), Typed(setType), left, right), nil)
	if !EqualTypedExpr(intersection, set(i(2))) {
		t.Fatalf("intersection = %v", intersection)
	}
	cardinality, _ := SimplifyConstants(MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), left), nil)
	if !EqualTypedExpr(cardinality, i(2)) {
		t.Fatalf("cardinality = %v", cardinality)
	}
	empty := set()
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), empty, NewNameExpr("P", Typed(BoolT1)))
	quantified, _ := SimplifyConstants(exists, nil)
	if !EqualTypedExpr(quantified, b(false)) {
		t.Fatalf("exists empty = %v", quantified)
	}

	rangeExpr := func(lower, upper int64) Expr {
		return MustOperExpr(Op("INT_RANGE"), Typed(setType), i(lower), i(upper))
	}
	for _, test := range []struct {
		name     string
		operator string
		domain   Expr
		body     bool
		want     bool
	}{
		{"exists true nonempty range", "EXISTS3", rangeExpr(1, 10000), true, true},
		{"exists false nonempty range", "EXISTS3", rangeExpr(1, 10000), false, false},
		{"forall true nonempty range", "FORALL3", rangeExpr(1, 10000), true, true},
		{"forall false nonempty range", "FORALL3", rangeExpr(1, 10000), false, false},
		{"exists empty range", "EXISTS3", rangeExpr(2, 1), true, false},
		{"forall empty range", "FORALL3", rangeExpr(2, 1), false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			quantifier := MustOperExpr(Op(test.operator), Typed(BoolT1),
				NewNameExpr("x", Typed(IntT1)), test.domain, b(test.body))
			got, simplifyErr := SimplifyConstants(quantifier, nil)
			if simplifyErr != nil || !EqualTypedExpr(got, b(test.want)) {
				t.Fatalf("SimplifyConstants() = %v, %v; want %v", got, simplifyErr, test.want)
			}
		})
	}
}

func TestSimplifyConstantsScalaLiteralOrderingAndSetIdentities(t *testing.T) {
	b := func(value bool) Expr { return NewValueExpr(BoolValue(value), Typed(BoolT1)) }
	s := func(value string) Expr { return NewValueExpr(StringValue(value), Typed(StrT1)) }
	mixedType := mustSet(IntT1)
	// The IR permits mixed literal values here for exercising Scala Literal's
	// total ordering: Bool < String < Int. The output tag is carried unchanged.
	set := func(values ...Expr) Expr { return MustOperExpr(Op("SET_ENUM"), Typed(mixedType), values...) }
	left := set(i(2), s("b"), b(true), s("a"), b(false), i(1), i(2))
	right := set(s("c"), b(true), i(3))
	union, err := SimplifyConstants(MustOperExpr(Op("SET_UNION2"), Typed(mixedType), left, right), nil)
	wantUnion := set(b(false), b(true), s("a"), s("b"), s("c"), i(1), i(2), i(3))
	if err != nil || !EqualTypedExpr(union, wantUnion) {
		t.Fatalf("mixed literal union = %v, %v; want %v", union, err, wantUnion)
	}
	difference, err := SimplifyConstants(MustOperExpr(Op("SET_MINUS"), Typed(mixedType), left, right), nil)
	wantDifference := set(b(false), s("a"), s("b"), i(1), i(2))
	if err != nil || !EqualTypedExpr(difference, wantDifference) {
		t.Fatalf("mixed literal difference = %v, %v; want %v", difference, err, wantDifference)
	}
	intersection, err := SimplifyConstants(MustOperExpr(Op("SET_INTERSECT"), Typed(mixedType), left, right), nil)
	wantIntersection := set(b(true))
	if err != nil || !EqualTypedExpr(intersection, wantIntersection) {
		t.Fatalf("mixed literal intersection = %v, %v; want %v", intersection, err, wantIntersection)
	}

	symbolic := NewNameExpr("S", Typed(mixedType))
	empty := set()
	identityCases := []struct{ input, want Expr }{
		{MustOperExpr(Op("SET_IN"), Typed(BoolT1), i(1), empty), b(false)},
		{MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), empty, symbolic), b(true)},
		{MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), symbolic, symbolic), b(true)},
		{MustOperExpr(Op("SET_UNION2"), Typed(mixedType), empty, symbolic), symbolic},
		{MustOperExpr(Op("SET_INTERSECT"), Typed(mixedType), symbolic, empty), empty},
		{MustOperExpr(Op("SET_MINUS"), Typed(mixedType), symbolic, empty), symbolic},
		{MustOperExpr(Op("SET_MINUS"), Typed(mixedType), symbolic, symbolic), empty},
	}
	for _, test := range identityCases {
		got, simplifyErr := SimplifyConstants(test.input, nil)
		if simplifyErr != nil || !EqualTypedExpr(got, test.want) {
			t.Errorf("set identity %v = %v, %v; want %v", test.input, got, simplifyErr, test.want)
		}
	}
}

func TestSimplifyConstantsScalaBooleanAndSymbolicQuantifierRules(t *testing.T) {
	b := func(value bool) Expr { return NewValueExpr(BoolValue(value), Typed(BoolT1)) }
	p := NewNameExpr("p", Typed(BoolT1))
	q := NewNameExpr("q", Typed(BoolT1))
	booleanCases := []struct{ input, want Expr }{
		{MustOperExpr(Op("IMPLIES"), Typed(BoolT1), b(false), p), b(true)},
		{MustOperExpr(Op("IMPLIES"), Typed(BoolT1), p, b(true)), b(true)},
		{MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), p, b(false), q), MustOperExpr(Op("AND"), Typed(BoolT1), MustOperExpr(Op("NOT"), Typed(BoolT1), p), q)},
		{MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), p, b(true), q), MustOperExpr(Op("OR"), Typed(BoolT1), p, q)},
	}
	for _, test := range booleanCases {
		got, err := SimplifyConstants(test.input, nil)
		if err != nil || !EqualTypedExpr(got, test.want) {
			t.Errorf("Boolean simplification %v = %v, %v; want %v", test.input, got, err, test.want)
		}
	}

	setType := mustSet(IntT1)
	domain := NewNameExpr("S", Typed(setType))
	x := NewNameExpr("x", Typed(IntT1))
	empty := MustOperExpr(Op("SET_ENUM"), Typed(setType))
	for _, test := range []struct {
		operator string
		body     bool
		want     Expr
	}{
		{"EXISTS3", true, MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EQ"), Typed(BoolT1), domain, empty))},
		{"EXISTS3", false, b(false)},
		{"FORALL3", true, b(true)},
		{"FORALL3", false, MustOperExpr(Op("EQ"), Typed(BoolT1), domain, empty)},
	} {
		got, err := SimplifyConstants(MustOperExpr(Op(test.operator), Typed(BoolT1), x, domain, b(test.body)), nil)
		if err != nil || !EqualTypedExpr(got, test.want) {
			t.Errorf("%s symbolic %v = %v, %v; want %v", test.operator, test.body, got, err, test.want)
		}
	}
}

func TestSimplifyConstantsRecursesThroughLetAndTracksSources(t *testing.T) {
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	local := NewOperDecl("Local", nil, MustOperExpr(Op("PLUS"), Typed(IntT1), one, NewValueExpr(Int64Value(0), Typed(IntT1))), Typed(IntT1))
	root := NewLetInExpr(MustOperExpr(Op("MINUS"), Typed(IntT1), one, one), Typed(IntT1), local)
	store := NewSourceStore()
	location := SourceLocation{Filename: "M", Region: NewSourceRegion(1, 1, 1, 20)}
	for _, expression := range []Expr{root, root.Body(), local.Body()} {
		if err := store.AddRecursive(expression, location); err != nil {
			t.Fatal(err)
		}
	}
	got, err := SimplifyConstants(root, NewListenerTracker(store))
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	if !EqualTypedExpr(let.Body(), NewValueExpr(Int64Value(0), Typed(IntT1))) || !EqualTypedExpr(let.Declarations()[0].Body(), one) {
		t.Fatalf("simplified LET = %v", got)
	}
	if !store.Contains(got.ID()) || !store.Contains(let.Body().ID()) {
		t.Fatal("simplification lost source locations")
	}
}

func TestSimplifyConstantsScalaGeneratedLawMatrix(t *testing.T) {
	i := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	b := func(value bool) Expr { return NewValueExpr(BoolValue(value), Typed(BoolT1)) }
	simplify := func(t *testing.T, expression Expr) Expr {
		t.Helper()
		result, err := SimplifyConstants(expression, nil)
		if err != nil {
			t.Fatalf("simplify %v: %v", expression, err)
		}
		return result
	}
	assertSimplifies := func(t *testing.T, input, want Expr) {
		t.Helper()
		got, expected := simplify(t, input), simplify(t, want)
		if !EqualTypedExpr(got, expected) {
			t.Fatalf("simplify %v = %v, want %v", input, got, expected)
		}
	}

	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	integers := []Expr{i(-2), i(-1), i(0), i(1), i(2), x, y,
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, i(2)),
		MustOperExpr(Op("MINUS"), Typed(IntT1), y, i(3)),
		MustOperExpr(Op("MULT"), Typed(IntT1), x, y)}
	for index, expression := range integers {
		t.Run(fmt.Sprintf("integer_laws_%d", index), func(t *testing.T) {
			for _, law := range []struct{ input, want Expr }{
				{MustOperExpr(Op("PLUS"), Typed(IntT1), i(0), expression), expression},
				{MustOperExpr(Op("PLUS"), Typed(IntT1), expression, i(0)), expression},
				{MustOperExpr(Op("MINUS"), Typed(IntT1), expression, i(0)), expression},
				{MustOperExpr(Op("MINUS"), Typed(IntT1), expression, expression), i(0)},
				{MustOperExpr(Op("MULT"), Typed(IntT1), expression, i(1)), expression},
				{MustOperExpr(Op("MULT"), Typed(IntT1), i(0), expression), i(0)},
				{MustOperExpr(Op("DIV"), Typed(IntT1), expression, i(1)), expression},
				{MustOperExpr(Op("MOD"), Typed(IntT1), expression, i(1)), i(0)},
				{MustOperExpr(Op("POW"), Typed(IntT1), expression, i(1)), expression},
			} {
				assertSimplifies(t, law.input, law.want)
			}
		})
	}

	for left := int64(-24); left <= 24; left++ {
		for right := int64(-24); right <= 24; right++ {
			for operator, want := range map[string]*big.Int{
				"PLUS":  new(big.Int).Add(big.NewInt(left), big.NewInt(right)),
				"MINUS": new(big.Int).Sub(big.NewInt(left), big.NewInt(right)),
				"MULT":  new(big.Int).Mul(big.NewInt(left), big.NewInt(right)),
			} {
				assertSimplifies(t, MustOperExpr(Op(operator), Typed(IntT1), i(left), i(right)),
					NewValueExpr(NewIntValue(want), Typed(IntT1)))
			}
			for operator, want := range map[string]bool{
				"LT": left < right, "LE": left <= right, "GT": left > right,
				"GE": left >= right, "EQ": left == right, "NE": left != right,
			} {
				assertSimplifies(t, MustOperExpr(Op(operator), Typed(BoolT1), i(left), i(right)), b(want))
			}
			if right != 0 {
				assertSimplifies(t, MustOperExpr(Op("DIV"), Typed(IntT1), i(left), i(right)), i(left/right))
				assertSimplifies(t, MustOperExpr(Op("MOD"), Typed(IntT1), i(left), i(right)), i(left%right))
			}
		}
	}

	p := NewNameExpr("p", Typed(BoolT1))
	q := NewNameExpr("q", Typed(BoolT1))
	booleans := []Expr{b(false), b(true), p, q,
		MustOperExpr(Op("AND"), Typed(BoolT1), p, q),
		MustOperExpr(Op("OR"), Typed(BoolT1), p, q),
		MustOperExpr(Op("NOT"), Typed(BoolT1), p)}
	for index, expression := range booleans {
		t.Run(fmt.Sprintf("Boolean_laws_%d", index), func(t *testing.T) {
			for _, law := range []struct{ input, want Expr }{
				{MustOperExpr(Op("EQ"), Typed(BoolT1), expression, expression), b(true)},
				{MustOperExpr(Op("NE"), Typed(BoolT1), expression, expression), b(false)},
				{MustOperExpr(Op("AND"), Typed(BoolT1), expression, b(true)), expression},
				{MustOperExpr(Op("AND"), Typed(BoolT1), expression, b(false)), b(false)},
				{MustOperExpr(Op("OR"), Typed(BoolT1), expression, b(false)), expression},
				{MustOperExpr(Op("OR"), Typed(BoolT1), expression, b(true)), b(true)},
				{MustOperExpr(Op("IMPLIES"), Typed(BoolT1), expression, b(false)), MustOperExpr(Op("NOT"), Typed(BoolT1), expression)},
				{MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), expression, b(true), b(false)), expression},
				{MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("NOT"), Typed(BoolT1), expression)), expression},
			} {
				assertSimplifies(t, law.input, law.want)
			}
		})
	}

	for _, left := range []string{"", "a", "b", "alpha", "β"} {
		for _, right := range []string{"", "a", "b", "alpha", "β"} {
			leftExpr := NewValueExpr(StringValue(left), Typed(StrT1))
			rightExpr := NewValueExpr(StringValue(right), Typed(StrT1))
			assertSimplifies(t, MustOperExpr(Op("EQ"), Typed(BoolT1), leftExpr, rightExpr), b(left == right))
			assertSimplifies(t, MustOperExpr(Op("NE"), Typed(BoolT1), leftExpr, rightExpr), b(left != right))
		}
	}
}
