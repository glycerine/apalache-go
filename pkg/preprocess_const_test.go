package apalache

import (
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
	for _, test := range []struct{ operator, message string }{{"DIV", "division by zero"}, {"MOD", "mod by zero"}, {"POW", "0 ^ 0"}} {
		_, err := SimplifyConstants(MustOperExpr(Op(test.operator), Typed(IntT1), one, zero), nil)
		if test.operator == "POW" {
			_, err = SimplifyConstants(MustOperExpr(Op(test.operator), Typed(IntT1), zero, zero), nil)
		}
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatalf("%s error = %v", test.operator, err)
		}
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
