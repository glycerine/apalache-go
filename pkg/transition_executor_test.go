package apalache

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestTransitionExecutorBuildsMultiStepPath(t *testing.T) {
	executor := NewTransitionExecutor(nil, []string{"x"})
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	init := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, intLiteral(0))
	enabled, err := executor.PrepareTransition(0, init)
	if err != nil || !enabled {
		t.Fatalf("prepare init = %v, %v", enabled, err)
	}
	if !reflect.DeepEqual(executor.PreparedTransitionNumbers(), []int{0}) {
		t.Fatalf("prepared transitions = %v", executor.PreparedTransitionNumbers())
	}
	if err = executor.AssumeTransition(0); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := executor.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("initial path = %v, %v", status, checkErr)
	}
	if err = executor.NextState(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(executor.StateTransitionNumbers(), []int{0}) {
		t.Fatalf("initial transition path = %v", executor.StateTransitionNumbers())
	}
	if executor.Step() != 1 || !executor.Binding().Contains("x") || executor.Binding().Contains("x'") {
		t.Fatalf("shifted state: step=%d binding=%v", executor.Step(), executor.Binding().Map())
	}

	nextValue := MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1))
	next := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, nextValue)
	if enabled, err = executor.PrepareTransition(3, next); err != nil || !enabled {
		t.Fatalf("prepare next = %v, %v", enabled, err)
	}
	if err = executor.AssumeTransition(3); err != nil {
		t.Fatal(err)
	}
	if err = executor.NextState(); err != nil {
		t.Fatal(err)
	}
	encoded := executor.EncodedExecution()
	if encoded.Arena != executor.scope.Arena || len(encoded.Path) != 2 ||
		encoded.Path[0].TransitionNumber != 0 || encoded.Path[1].TransitionNumber != 3 {
		t.Fatalf("encoded execution = %#v", encoded)
	}
	equalsOne := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1))
	if err = executor.AssertState(equalsOne); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := executor.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
		t.Fatalf("two-step path = %v, %v", status, checkErr)
	}
	if err = executor.AssertState(MustOperExpr(Op("NOT"), Typed(BoolT1), equalsOne)); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := executor.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("contradictory path = %v, %v", status, checkErr)
	}
}

func TestTransitionExecutorDecodedExecutionRetainsTransitionNumbers(t *testing.T) {
	executor := NewTransitionExecutor(nil, []string{"x"})
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	assign := func(value int64) Expr {
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, intLiteral(value))
	}
	if enabled, err := executor.PrepareTransition(11, assign(1)); err != nil || !enabled {
		t.Fatalf("prepare initial = %v, %v", enabled, err)
	}
	if err := executor.AssumeTransition(11); err != nil {
		t.Fatal(err)
	}
	if err := executor.NextState(); err != nil {
		t.Fatal(err)
	}
	firstStateSnapshot := executor.Snapshot()
	if enabled, err := executor.PrepareTransition(22, assign(2)); err != nil || !enabled {
		t.Fatalf("prepare pending = %v, %v", enabled, err)
	}
	if err := executor.AssumeTransition(22); err != nil {
		t.Fatal(err)
	}
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	status, pending, err := executor.CheckPendingAndDecode(context.Background(), truth)
	if err != nil || status != SMTSatisfiable || len(pending) != 2 ||
		pending[0].TransitionNumber != 11 || pending[1].TransitionNumber != 22 {
		t.Fatalf("pending decoded execution = %v, %#v, %v", status, pending, err)
	}
	if err := executor.NextState(); err != nil {
		t.Fatal(err)
	}
	status, execution, err := executor.CheckDecodedExecution(context.Background(), truth)
	if err != nil || status != SMTSatisfiable || len(execution.Path) != 2 ||
		execution.Path[0].TransitionNumber != 11 || execution.Path[1].TransitionNumber != 22 {
		t.Fatalf("decoded execution = %v, %#v, %v", status, execution, err)
	}
	executor.Recover(firstStateSnapshot)
	if executor.Step() != 1 || !reflect.DeepEqual(executor.StateTransitionNumbers(), []int{11}) ||
		len(executor.EncodedExecution().Path) != 1 {
		t.Fatalf("recovered encoded execution = step %d, transitions %v, path %#v",
			executor.Step(), executor.StateTransitionNumbers(), executor.EncodedExecution().Path)
	}
}

func TestTransitionExecutorConstantsAlternativesAndPartialAssignments(t *testing.T) {
	executor := NewTransitionExecutor([]string{"N"}, []string{"x", "y"})
	constantPrime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("N", Typed(IntT1)))
	constantInit := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), constantPrime, intLiteral(4))
	if err := executor.InitializeConstants(constantInit); err != nil {
		t.Fatal(err)
	}

	xPrime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	partial := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), xPrime, NewNameExpr("N", Typed(IntT1)))
	if enabled, err := executor.PrepareTransition(0, partial); err != nil || enabled {
		t.Fatalf("partial transition = %v, %v", enabled, err)
	}
	if len(executor.PreparedTransitionNumbers()) != 0 {
		t.Fatalf("partial transition was retained: %v", executor.PreparedTransitionNumbers())
	}

	yPrime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("y", Typed(IntT1)))
	branch := func(value int64) Expr {
		return MustOperExpr(Op("AND"), Typed(BoolT1),
			MustOperExpr(Op("Apalache!:="), Typed(BoolT1), xPrime, intLiteral(value)),
			MustOperExpr(Op("Apalache!:="), Typed(BoolT1), yPrime, intLiteral(value+1)))
	}
	if enabled, err := executor.PrepareTransition(7, branch(10)); err != nil || !enabled {
		t.Fatalf("first complete branch = %v, %v", enabled, err)
	}
	if enabled, err := executor.PrepareTransition(2, branch(20)); err != nil || !enabled {
		t.Fatalf("second complete branch = %v, %v", enabled, err)
	}
	if !reflect.DeepEqual(executor.PreparedTransitionNumbers(), []int{2, 7}) {
		t.Fatalf("prepared transition ordering = %v", executor.PreparedTransitionNumbers())
	}
	if err := executor.AssumeTransition(2); err != nil {
		t.Fatal(err)
	}
	if err := executor.NextState(); err != nil {
		t.Fatal(err)
	}
	claim := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("N", Typed(IntT1)), intLiteral(4)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(20)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("y", Typed(IntT1)), intLiteral(21)))
	if err := executor.AssertState(claim); err != nil {
		t.Fatal(err)
	}
	if status, err := executor.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("selected alternative = %v, %v", status, err)
	}
}

func TestTransitionExecutorCheckStateIsTemporary(t *testing.T) {
	executor := NewTransitionExecutor(nil, []string{"x"})
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	init := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, intLiteral(0))
	if enabled, err := executor.PrepareTransition(0, init); err != nil || !enabled {
		t.Fatalf("prepare init = %v, %v", enabled, err)
	}
	if err := executor.AssumeTransition(0); err != nil {
		t.Fatal(err)
	}
	if err := executor.NextState(); err != nil {
		t.Fatal(err)
	}
	equals := func(value int64) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(value))
	}
	if status, err := executor.CheckState(context.Background(), equals(0)); err != nil || status != SMTSatisfiable {
		t.Fatalf("temporary true state = %v, %v", status, err)
	}
	if status, err := executor.CheckState(context.Background(), equals(1)); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("temporary false state = %v, %v", status, err)
	}
	if status, err := executor.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("temporary checks changed path = %v, %v", status, err)
	}
}

func TestTransitionExecutorSolvesSetMembershipWithNativeArrays(t *testing.T) {
	executor := newTransitionExecutorWithEncodingAndSolverOptions(
		nil, nil, nil, nil, false, SMTEncodingArrays)
	set := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1))
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), intLiteral(1), set)
	if err := executor.AssertState(membership); err != nil {
		t.Fatal(err)
	}
	solver, encoder, status, err := executor.solve(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if status != SMTSatisfiable || encoder.encoding != SMTEncodingArrays || len(encoder.memberships) != 0 {
		t.Fatalf("native array solve = %v, encoding=%s, memberships=%v", status, encoder.encoding, encoder.memberships)
	}
	if err = executor.AssertState(MustOperExpr(Op("NOT"), Typed(BoolT1), membership)); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := executor.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("contradictory native membership = %v, %v", status, checkErr)
	}
}

func TestTransitionExecutorSnapshotRecoversPickedPathAndIsReusable(t *testing.T) {
	executor := NewTransitionExecutor(nil, []string{"x"})
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	init := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, intLiteral(5))
	if enabled, err := executor.PrepareTransition(7, init); err != nil || !enabled {
		t.Fatalf("prepare init = %v, %v", enabled, err)
	}
	if err := executor.AssumeTransition(7); err != nil {
		t.Fatal(err)
	}
	snapshot := executor.Snapshot()
	if snapshot.ControlState != ExecutorPicked || executor.ControlState() != ExecutorPicked {
		t.Fatalf("snapshot control state = %v, executor = %v", snapshot.ControlState, executor.ControlState())
	}

	contradiction := NewValueExpr(BoolValue(false), Typed(BoolT1))
	if err := executor.AssertState(contradiction); err != nil {
		t.Fatal(err)
	}
	if err := executor.NextState(); err != nil {
		t.Fatal(err)
	}
	if status, err := executor.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("mutated path = %v, %v", status, err)
	}

	for recovery := 0; recovery < 2; recovery++ {
		executor.Recover(snapshot)
		if executor.Step() != 0 || executor.ControlState() != ExecutorPicked ||
			!reflect.DeepEqual(executor.PreparedTransitionNumbers(), []int{7}) ||
			len(executor.StateBindings()) != 0 || !executor.Binding().Contains("x'") {
			t.Fatalf("recovered executor = step %d, state %v, prepared %v, path %d, binding %v",
				executor.Step(), executor.ControlState(), executor.PreparedTransitionNumbers(),
				len(executor.StateBindings()), executor.Binding().Map())
		}
		if status, err := executor.Check(context.Background()); err != nil || status != SMTSatisfiable {
			t.Fatalf("recovered path %d = %v, %v", recovery, status, err)
		}
		// Mutate after each recovery. A second recovery from the same snapshot
		// proves the snapshot does not alias executor-owned slices or maps.
		if err := executor.AssertState(contradiction); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTransitionExecutorRejectsInvalidSnapshot(t *testing.T) {
	executor := NewTransitionExecutor(nil, nil)
	if err := executor.RecoverChecked(TransitionExecutorSnapshot{}); err == nil {
		t.Fatal("snapshot without arena was accepted")
	}
	snapshot := executor.Snapshot()
	snapshot.ControlState = ExecutorControlState(99)
	if err := executor.RecoverChecked(snapshot); err == nil {
		t.Fatal("snapshot with invalid control state was accepted")
	}
	snapshot = executor.Snapshot()
	snapshot.States = append(snapshot.States, EmptyBinding())
	if err := executor.RecoverChecked(snapshot); err == nil {
		t.Fatal("snapshot with mismatched transition history was accepted")
	}
}

func TestTransitionExecutorMayChangeAssertionMatchesScala(t *testing.T) {
	executor := NewTransitionExecutor(nil, []string{"x", "y"})
	scope := InitialRewriterScope()
	var x, y, nextY ArenaCell
	var err error
	scope, x, err = executor.rewriter.Rewrite(intLiteral(1), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope, y, err = executor.rewriter.Rewrite(intLiteral(2), scope)
	if err != nil {
		t.Fatal(err)
	}
	_, nextY, err = executor.rewriter.Rewrite(intLiteral(3), scope)
	if err != nil {
		t.Fatal(err)
	}
	executor.prepared[7] = EncodedTransition{Binding: BindingOf(
		BindingEntry{Name: "x", Cell: x}, BindingEntry{Name: "x'", Cell: x},
		BindingEntry{Name: "y", Cell: y}, BindingEntry{Name: "y'", Cell: nextY},
	)}
	xAssertion := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1))
	yPrimeAssertion := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("y", Typed(IntT1))), intLiteral(3))

	if changed, err := executor.MayChangeAssertion(7, StateInvariant, 0, xAssertion); err != nil || !changed {
		t.Fatalf("initial-step assertion change = %v, %v", changed, err)
	}
	executor.step = 1
	if changed, err := executor.MayChangeAssertion(7, StateInvariant, 0, xAssertion); err != nil || changed {
		t.Fatalf("unchanged x assertion = %v, %v", changed, err)
	}
	if changed, err := executor.MayChangeAssertion(7, ActionInvariant, 1, yPrimeAssertion); err != nil || !changed {
		t.Fatalf("changed y' assertion = %v, %v", changed, err)
	}
	if _, err := executor.MayChangeAssertion(8, StateInvariant, 0, xAssertion); err == nil {
		t.Fatal("unprepared transition was accepted")
	}
	if _, err := executor.MayChangeAssertion(7, StateInvariant, 0, nil); err == nil {
		t.Fatal("nil assertion was accepted")
	}
}

func TestTransitionExecutorPickTransitionPreservesSparseChoiceAndBinding(t *testing.T) {
	executor := NewTransitionExecutor(nil, []string{"x"})
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	assign := func(value int64) Expr {
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, intLiteral(value))
	}
	for number, value := range map[int]int64{2: 10, 7: 20} {
		if enabled, err := executor.PrepareTransition(number, assign(value)); err != nil || !enabled {
			t.Fatalf("prepare %d = %v, %v", number, enabled, err)
		}
	}
	oracle, err := executor.PickTransition()
	if err != nil {
		t.Fatal(err)
	}
	sparse, ok := oracle.(*SparseOracle)
	if !ok || !reflect.DeepEqual(sparse.Values, []int{2, 7}) {
		t.Fatalf("pick oracle = %#v", oracle)
	}
	forceSeven := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x'", Typed(IntT1)), intLiteral(20))
	if err := executor.AssertState(forceSeven); err != nil {
		t.Fatal(err)
	}
	status, pending, err := executor.CheckPendingAndDecode(context.Background(), boolExpr(true))
	if err != nil || status != SMTSatisfiable || len(pending) != 1 || pending[0].TransitionNumber != 7 ||
		pending[0].Values["x"].(*ValueExpr).Value().(IntValue).BigInt().Int64() != 20 {
		t.Fatalf("pending sparse choice = %v, %#v, %v", status, pending, err)
	}
	if err := executor.NextState(); err != nil {
		t.Fatal(err)
	}
	status, execution, err := executor.CheckDecodedExecution(context.Background(), boolExpr(true))
	if err != nil || status != SMTSatisfiable || len(execution.Path) != 1 || execution.Path[0].TransitionNumber != 7 {
		t.Fatalf("committed sparse choice = %v, %#v, %v", status, execution, err)
	}
}

func TestTransitionExecutorPickTransitionEmptyAndSingleton(t *testing.T) {
	empty := NewTransitionExecutor(nil, nil)
	if _, err := empty.PickTransition(); err == nil {
		t.Fatal("empty transition set was picked")
	}
	single := NewTransitionExecutor(nil, nil)
	if enabled, err := single.PrepareTransition(9, boolExpr(true)); err != nil || !enabled {
		t.Fatalf("prepare singleton = %v, %v", enabled, err)
	}
	oracle, err := single.PickTransition()
	if err != nil {
		t.Fatal(err)
	}
	if fixed, ok := oracle.(*MockOracle); !ok || fixed.Fixed != 9 {
		t.Fatalf("singleton oracle = %#v", oracle)
	}
	if _, err := single.PickTransition(); err == nil {
		t.Fatal("second transition pick was accepted")
	}
	status, states, err := single.CheckPendingAndDecode(context.Background(), boolExpr(true))
	if err != nil || status != SMTSatisfiable || len(states) != 1 || states[0].TransitionNumber != 9 {
		t.Fatalf("singleton decoded pick = %v, %#v, %v", status, states, err)
	}
}

func TestTransitionExecutorEvaluatesCurrentModel(t *testing.T) {
	executor := NewTransitionExecutor(nil, []string{"x"})
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	initial := MustOperExpr(Op("Apalache!:="), Typed(BoolT1),
		prime, NewValueExpr(Int64Value(7), Typed(IntT1)))
	enabled, err := executor.PrepareTransition(0, initial)
	if err != nil || !enabled {
		t.Fatalf("prepare = %v, %v", enabled, err)
	}
	if err = executor.AssumeTransition(0); err != nil {
		t.Fatal(err)
	}
	if err = executor.NextState(); err != nil {
		t.Fatal(err)
	}
	value, status, err := executor.Evaluate(context.Background(), NewNameExpr("x", Typed(IntT1)))
	if err != nil || status != SMTSatisfiable || !EqualValues(value.(*ValueExpr).Value(), Int64Value(7)) {
		t.Fatalf("Evaluate(x) = %v, %v, %v", value, status, err)
	}
}

func TestTransitionExecutorRejectsInvalidLifecycle(t *testing.T) {
	executor := NewTransitionExecutor(nil, []string{"x"})
	if err := executor.NextState(); err == nil || !strings.Contains(err.Error(), "select") {
		t.Fatalf("next without selection = %v", err)
	}
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	if err := executor.AssumeTransition(9); err == nil || !strings.Contains(err.Error(), "prepared") {
		t.Fatalf("unknown transition selection = %v", err)
	}
	if err := executor.AssertState(intLiteral(1)); err == nil || !strings.Contains(err.Error(), "Boolean type") {
		t.Fatalf("integer assertion = %v", err)
	}
	if err := executor.InitializeConstants(truth); err != nil {
		t.Fatal(err)
	}
}
