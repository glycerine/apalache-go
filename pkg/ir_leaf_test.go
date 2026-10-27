package apalache

import (
	"math/big"
	"sync"
	"testing"
)

func TestUIDsAreUniqueConcurrently(t *testing.T) {
	const workers = 8
	const perWorker = 1_000
	ids := make(chan UID, workers*perWorker)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for range perWorker {
				ids <- NewUID()
			}
		}()
	}
	group.Wait()
	close(ids)
	seen := make(map[UID]bool, workers*perWorker)
	for id := range ids {
		if id == NullUID() || seen[id] {
			t.Fatalf("invalid or duplicate UID %v", id)
		}
		seen[id] = true
	}
	if len(seen) != workers*perWorker {
		t.Fatalf("got %d UIDs, want %d", len(seen), workers*perWorker)
	}
}

func TestTypeTags(t *testing.T) {
	untyped := Untyped()
	if untyped.IsTyped() || untyped.String() != "Untyped" {
		t.Fatalf("untyped tag = %v", untyped)
	}
	typed := Typed(IntT1)
	got, ok := typed.Type()
	if !ok || !EqualTypes(got, IntT1) || !typed.Equal(Typed(IntT1)) {
		t.Fatalf("typed tag = %v", typed)
	}
	if typed.Equal(Typed(BoolT1)) || typed.Equal(untyped) {
		t.Fatal("different tags compare equal")
	}
}

func TestLiteralValues(t *testing.T) {
	original := big.NewInt(42)
	integer := NewIntValue(original)
	original.SetInt64(-1)
	if integer.String() != "42" || !integer.IsNatural() {
		t.Fatalf("integer changed through input alias: %s", integer)
	}
	copy := integer.BigInt()
	copy.SetInt64(-1)
	if integer.String() != "42" {
		t.Fatalf("integer changed through output alias: %s", integer)
	}
	if Int64Value(-1).IsNatural() || !Int64Value(0).IsNatural() {
		t.Fatal("natural-number classification is wrong")
	}
	if !EqualValues(integer, Int64Value(42)) || EqualValues(integer, Int64Value(41)) {
		t.Fatal("integer equality is wrong")
	}

	decimal, err := NewDecimalValue("12.50")
	if err != nil || decimal.String() != "25/2" {
		t.Fatalf("decimal = (%v, %v)", decimal, err)
	}
	if _, err := NewDecimalValue("not-a-decimal"); err == nil {
		t.Fatal("invalid decimal accepted")
	}
	if !EqualValues(BoolValue(true), BoolValue(true)) || EqualValues(BoolValue(true), BoolValue(false)) {
		t.Fatal("Boolean equality is wrong")
	}
	if StringValue("hello").Value() != "hello" {
		t.Fatal("string value is wrong")
	}
}

func TestPredefinedSetNames(t *testing.T) {
	tests := map[PredefinedSet]string{
		BoolSet: "BOOLEAN", StringSet: "STRING", IntSet: "Int", NatSet: "Nat", RealSet: "Real",
	}
	for set, want := range tests {
		if got := set.Name(); got != want {
			t.Errorf("Name() = %q, want %q", got, want)
		}
	}
}

func TestOperatorParam(t *testing.T) {
	if ValueParam("x").IsOperator() {
		t.Fatal("value parameter classified as operator")
	}
	if !(OperatorParam{Name: "F", Arity: 2}).IsOperator() {
		t.Fatal("operator parameter classified as value")
	}
}

func TestSourcePositionOrdering(t *testing.T) {
	positions := []SourcePosition{{0, 0}, {15, 20}, {15, 55}, {19, 5}, {5454, 4646}}
	for i := 1; i < len(positions); i++ {
		if positions[i-1].Compare(positions[i]) >= 0 {
			t.Fatalf("%v is not before %v", positions[i-1], positions[i])
		}
	}
	if (SourcePosition{15, 20}).Compare(SourcePosition{15, 20}) != 0 {
		t.Fatal("equal positions compare nonzero")
	}
}

func TestSourceRegionsMatchScalaSemantics(t *testing.T) {
	root := NewSourceRegion(0, 0, 5454, 4646)
	r15 := NewSourceRegion(15, 20, 19, 20)
	r24 := NewSourceRegion(15, 20, 19, 5)
	r01 := NewSourceRegion(0, 0, 15, 20)
	r56 := NewSourceRegion(19, 20, 5454, 4646)

	if root.String() != "0:0-5454:4646" || r15.String() != "15:20-19:20" {
		t.Fatal("region formatting differs from Scala")
	}
	if !r15.IsInside(root) || !root.Contains(r24) || root.IsInside(r15) {
		t.Fatal("region containment is wrong")
	}
	if !r15.Intersects(r24) || !r01.Intersects(r15) || r01.Intersects(r56) {
		t.Fatal("inclusive region intersection is wrong")
	}
}

func TestSourceLocationFormatting(t *testing.T) {
	location := SourceLocation{Filename: "root", Region: NewSourceRegion(1, 2, 3, 4)}
	if got, want := location.String(), "root.tla:1:2-3:4"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
