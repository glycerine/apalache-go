package apalache

import (
	"errors"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestEmptyTLCConfigHasExplicitNullBehaviorAndOwnedCollections(t *testing.T) {
	first, second := EmptyTLCConfig(), EmptyTLCConfig()
	if first.Behavior != (NullSpec{}) || first.CheckDeadlock != nil || first.ConstAssignments == nil ||
		first.ConstReplacements == nil || first.StateConstraints == nil || first.ActionConstraints == nil ||
		first.Invariants == nil || first.TemporalProperties == nil {
		t.Fatalf("empty config = %#v", first)
	}
	first.ConstReplacements["A"] = "B"
	first.Invariants = append(first.Invariants, "Inv")
	if len(second.ConstReplacements) != 0 || len(second.Invariants) != 0 {
		t.Fatal("empty TLC configurations share mutable storage")
	}
}

func TestTLCConfigAddOperationsAreImmutableAndOrdered(t *testing.T) {
	integer := big.NewInt(10)
	base := EmptyTLCConfig().
		AddConstAssignments(map[string]ConfigConstExpr{"B": ConfigModelValue{Name: "Old"}}).
		AddConstReplacements(map[string]string{"R": "OldR"}).
		AddStateConstraints([]string{"S0"}).
		AddActionConstraints([]string{"A0"}).
		AddInvariants([]string{"I0"}).
		AddTemporalProperties([]string{"P0"})
	updated := base.
		AddConstAssignments(map[string]ConfigConstExpr{
			"B": ConfigModelValue{Name: "New"}, "A": ConfigIntValue{Value: integer},
		}).
		AddConstReplacements(map[string]string{"R": "NewR", "Q": "QDef"}).
		AddStateConstraints([]string{"S1"}).
		AddActionConstraints([]string{"A1"}).
		AddInvariants([]string{"I1"}).
		AddTemporalProperties([]string{"P1"})
	integer.SetInt64(99)
	if base.ConstAssignments["B"].(ConfigModelValue).Name != "Old" || base.ConstReplacements["R"] != "OldR" ||
		!reflect.DeepEqual(base.Invariants, []string{"I0"}) {
		t.Fatalf("base was mutated = %#v", base)
	}
	if updated.ConstAssignments["B"].(ConfigModelValue).Name != "New" ||
		updated.ConstAssignments["A"].(ConfigIntValue).Value.Int64() != 10 || updated.ConstReplacements["R"] != "NewR" ||
		!reflect.DeepEqual(updated.ConstAssignmentOrder, []string{"B", "A"}) ||
		!reflect.DeepEqual(updated.ConstReplacementOrder, []string{"R", "Q"}) ||
		!reflect.DeepEqual(updated.StateConstraints, []string{"S0", "S1"}) ||
		!reflect.DeepEqual(updated.ActionConstraints, []string{"A0", "A1"}) ||
		!reflect.DeepEqual(updated.Invariants, []string{"I0", "I1"}) ||
		!reflect.DeepEqual(updated.TemporalProperties, []string{"P0", "P1"}) {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestTLCConfigBehaviorAndDeadlockSetUnlessUnset(t *testing.T) {
	base := EmptyTLCConfig()
	unchanged, err := base.SetBehaviorSpecUnlessNull(NullSpec{})
	if err != nil || unchanged.Behavior != (NullSpec{}) {
		t.Fatalf("null behavior = %#v, %v", unchanged.Behavior, err)
	}
	withBehavior, err := base.SetBehaviorSpecUnlessNull(InitNextSpec{Init: "Init", Next: "Next"})
	if err != nil || withBehavior.Behavior != (InitNextSpec{Init: "Init", Next: "Next"}) {
		t.Fatalf("behavior = %#v, %v", withBehavior.Behavior, err)
	}
	if _, err = withBehavior.SetBehaviorSpecUnlessNull(TemporalSpec{Name: "Spec"}); err == nil {
		t.Fatal("accepted two behavior specifications")
	} else {
		var parseError *TLCConfigParseError
		if !errors.As(err, &parseError) || !strings.Contains(parseError.Message, "Found several behavior specifications") {
			t.Fatalf("behavior conflict = %T %v", err, err)
		}
	}
	falseValue, trueValue := false, true
	withDeadlock, err := base.SetCheckDeadlockUnlessUnset(&falseValue)
	if err != nil || withDeadlock.CheckDeadlock == nil || *withDeadlock.CheckDeadlock {
		t.Fatalf("deadlock = %#v, %v", withDeadlock.CheckDeadlock, err)
	}
	if same, err := withDeadlock.SetCheckDeadlockUnlessUnset(&falseValue); err != nil || *same.CheckDeadlock {
		t.Fatalf("equal deadlock = %#v, %v", same.CheckDeadlock, err)
	}
	if _, err := withDeadlock.SetCheckDeadlockUnlessUnset(&trueValue); err == nil {
		t.Fatal("accepted conflicting CHECK_DEADLOCK values")
	}
}

func TestTLCConfigJoinUsesRightMapValuesAndConcatenatesLists(t *testing.T) {
	leftFalse, rightFalse := false, false
	left, err := EmptyTLCConfig().SetBehaviorSpecUnlessNull(InitNextSpec{Init: "Init", Next: "Next"})
	if err != nil {
		t.Fatal(err)
	}
	left = left.AddConstAssignments(map[string]ConfigConstExpr{"N": ConfigModelValue{Name: "Old"}}).AddInvariants([]string{"Left"})
	left.CheckDeadlock = &leftFalse
	right := EmptyTLCConfig().AddConstAssignmentsInOrder(map[string]ConfigConstExpr{
		"N": ConfigModelValue{Name: "New"}, "K": ConfigModelValue{Name: "K"},
	}, []string{"K", "N"}).AddInvariants([]string{"Right"})
	right.CheckDeadlock = &rightFalse
	joined, err := left.Join(right)
	if err != nil {
		t.Fatal(err)
	}
	if joined.ConstAssignments["N"].(ConfigModelValue).Name != "New" ||
		!reflect.DeepEqual(joined.ConstAssignmentOrder, []string{"N", "K"}) ||
		!reflect.DeepEqual(joined.Invariants, []string{"Left", "Right"}) ||
		joined.Behavior != (InitNextSpec{Init: "Init", Next: "Next"}) || joined.CheckDeadlock == nil || *joined.CheckDeadlock {
		t.Fatalf("joined = %#v", joined)
	}
	right.Behavior = TemporalSpec{Name: "Spec"}
	if _, err := left.Join(right); err == nil {
		t.Fatal("join accepted conflicting behaviors")
	}
}
