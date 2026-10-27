package apalache

import (
	"reflect"
	"testing"
)

func TestRenamedNameUtilities(t *testing.T) {
	name := MakeRenamedName("x", 42)
	base, counter, renamed, err := ParseRenamedName(name)
	if err != nil || !renamed || base != "x" || counter != 42 {
		t.Fatalf("parse %q = (%q, %d, %v, %v)", name, base, counter, renamed, err)
	}
	if _, _, renamed, err = ParseRenamedName("plain"); err != nil || renamed {
		t.Fatalf("plain name parsed as renamed: renamed=%v err=%v", renamed, err)
	}
	if _, _, _, err = ParseRenamedName("x$1$2"); err == nil {
		t.Fatal("accepted a name with multiple separators")
	}
	max := MergeNameCounters(map[string]int{"x": 1}, map[string]int{"x": 2, "y": 3}, true)
	min := MergeNameCounters(map[string]int{"x": 1}, map[string]int{"x": 2, "y": 3}, false)
	if !reflect.DeepEqual(max, map[string]int{"x": 2, "y": 3}) ||
		!reflect.DeepEqual(min, map[string]int{"x": 1, "y": 3}) {
		t.Fatalf("counter merges: max=%v min=%v", max, min)
	}
	if shifted, err := OffsetRenamedName(map[string]int{"x": 4}, "x$5"); err != nil || shifted != "x$1" {
		t.Fatalf("shift got %q, %v", shifted, err)
	}
}

func TestIncrementalRenamingQuantifiers(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	s := NewNameExpr("S", Untyped())
	one := NewValueExpr(Int64Value(1), Untyped())
	left := MustOperExpr(Op("EXISTS3"), Untyped(), x, s, MustOperExpr(Op("GT"), Untyped(), x, one))
	right := MustOperExpr(Op("FORALL3"), Untyped(), x, s, MustOperExpr(Op("LT"), Untyped(), x, one))
	input := MustOperExpr(Op("AND"), Untyped(), left, right)
	renamer := NewIncrementalRenamer(IdleTracker{})

	got := renamer.Rename(input).(*OperExpr).Arguments()
	assertBinderAndUse(t, got[0], "x$1")
	assertBinderAndUse(t, got[1], "x$2")
	gotAgain := renamer.Rename(MustOperExpr(Op("AND"), Untyped(), got...)).(*OperExpr).Arguments()
	assertBinderAndUse(t, gotAgain[0], "x$3")
	assertBinderAndUse(t, gotAgain[1], "x$4")
}

func assertBinderAndUse(t *testing.T, expression Expr, want string) {
	t.Helper()
	quantifier := expression.(*OperExpr)
	arguments := quantifier.Arguments()
	if arguments[0].(*NameExpr).Name() != want {
		t.Fatalf("binder is %q, want %q", arguments[0].(*NameExpr).Name(), want)
	}
	predicate := arguments[2].(*OperExpr)
	if predicate.Arguments()[0].(*NameExpr).Name() != want {
		t.Fatalf("use is %q, want %q", predicate.Arguments()[0].(*NameExpr).Name(), want)
	}
}

func TestIncrementalRenamingLetIn(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	s := NewNameExpr("S", Untyped())
	localBody := MustOperExpr(Op("FORALL3"), Untyped(), x, s,
		untypedApp("R", NewNameExpr("t", Untyped()), x))
	local := NewOperDecl("p", []OperatorParam{ValueParam("t")}, localBody, Untyped())
	body := MustOperExpr(Op("EXISTS3"), Untyped(), x, s, untypedApp("p", x))
	got := NewIncrementalRenamer(IdleTracker{}).Rename(NewLetInExpr(body, Untyped(), local)).(*LetInExpr)

	declaration := got.Declarations()[0]
	if declaration.Name() != "p$1" || declaration.FormalParams()[0].Name != "t$1" {
		t.Fatalf("LET declaration names: %s %v", declaration.Name(), declaration.FormalParams())
	}
	localQuantifier := declaration.Body().(*OperExpr)
	if localQuantifier.Arguments()[0].(*NameExpr).Name() != "x$1" {
		t.Fatalf("local binder = %s", localQuantifier.Arguments()[0].(*NameExpr).Name())
	}
	localCall := localQuantifier.Arguments()[2].(*OperExpr).Arguments()
	if localCall[1].(*NameExpr).Name() != "t$1" || localCall[2].(*NameExpr).Name() != "x$1" {
		t.Fatalf("local call was not renamed: %#v", localCall)
	}
	outer := got.Body().(*OperExpr)
	if outer.Arguments()[0].(*NameExpr).Name() != "x$2" {
		t.Fatalf("outer binder = %s", outer.Arguments()[0].(*NameExpr).Name())
	}
	call := outer.Arguments()[2].(*OperExpr).Arguments()
	if call[0].(*NameExpr).Name() != "p$1" || call[1].(*NameExpr).Name() != "x$2" {
		t.Fatalf("renamed call = %#v", call)
	}
}

func TestIncrementalRenamingNormalizeAndSync(t *testing.T) {
	x := NewNameExpr("x$5", Untyped())
	y := NewNameExpr("y$3", Untyped())
	z := NewNameExpr("z$4", Untyped())
	expression := MustOperExpr(Op("GT"), Untyped(), MustOperExpr(Op("PLUS"), Untyped(), x, y), z)
	renamer := NewIncrementalRenamer(IdleTracker{})
	normalized, err := renamer.Normalize(expression)
	if err != nil {
		t.Fatal(err)
	}
	want := MustOperExpr(Op("GT"), Untyped(), MustOperExpr(Op("PLUS"), Untyped(),
		NewNameExpr("x$1", Untyped()), NewNameExpr("y$1", Untyped())), NewNameExpr("z$1", Untyped()))
	if !EqualExpr(normalized, want) {
		t.Fatalf("normalized expression differs: %#v", normalized)
	}

	bound := MustOperExpr(Op("EXISTS3"), Untyped(), NewNameExpr("x$5", Untyped()),
		NewNameExpr("S", Untyped()), NewNameExpr("p", Untyped()))
	renamer.SyncFrom(bound)
	renamed := renamer.Rename(bound).(*OperExpr)
	if got := renamed.Arguments()[0].(*NameExpr).Name(); got != "x$6" {
		t.Fatalf("synced binder = %q, want x$6", got)
	}
}

func TestIncrementalRenamingMultipleBinders(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	y := NewNameExpr("y", Untyped())
	set := MustOperExpr(Op("SET_ENUM"), Untyped(), NewValueExpr(Int64Value(1), Untyped()))
	result := MustOperExpr(Op("PLUS"), Untyped(), x, y)
	input := MustOperExpr(Op("SET_MAP"), Untyped(), result, x, set, y, set)
	got := NewIncrementalRenamer(IdleTracker{}).Rename(input).(*OperExpr).Arguments()
	if got[1].(*NameExpr).Name() != "x$1" || got[3].(*NameExpr).Name() != "y$1" {
		t.Fatalf("binders not renamed: %#v", got)
	}
	resultGot := got[0].(*OperExpr).Arguments()
	if resultGot[0].(*NameExpr).Name() != "x$1" || resultGot[1].(*NameExpr).Name() != "y$1" {
		t.Fatalf("bound uses not renamed: %#v", resultGot)
	}
}

func TestIncrementalRenamingModuleNormalizesAsOneBatch(t *testing.T) {
	s := NewNameExpr("S", Untyped())
	p := NewNameExpr("p", Untyped())
	quantifier := func(name string) Expr {
		return MustOperExpr(Op("EXISTS3"), Untyped(), NewNameExpr(name, Untyped()), s, p)
	}
	renamer := NewIncrementalRenamer(IdleTracker{})
	renamer.SyncFrom(quantifier("x$99"))
	module := NewModule("M", []Decl{
		NewOperDecl("X", nil, quantifier("x$3"), Untyped()),
		NewOperDecl("Y", nil, quantifier("x$105"), Untyped()),
	})
	got, err := renamer.RenameModule(module)
	if err != nil {
		t.Fatal(err)
	}
	declarations := got.OperDeclarations()
	if first := declarations[0].Body().(*OperExpr).Arguments()[0].(*NameExpr).Name(); first != "x$1" {
		t.Fatalf("first normalized binder = %q", first)
	}
	if second := declarations[1].Body().(*OperExpr).Arguments()[0].(*NameExpr).Name(); second != "x$2" {
		t.Fatalf("second normalized binder = %q", second)
	}
}

func TestIncrementalRenamingNormalizationPreservesSource(t *testing.T) {
	store := NewSourceStore()
	original := NewNameExpr("x$3", Untyped())
	location := SourceLocation{Filename: "renaming", Region: NewSourceRegion(1, 1, 1, 4)}
	if err := store.Add(original.ID(), location); err != nil {
		t.Fatal(err)
	}
	renamer := NewIncrementalRenamer(NewListenerTracker(store))
	got, err := renamer.Normalize(original)
	if err != nil {
		t.Fatal(err)
	}
	if got.(*NameExpr).Name() != "x$1" {
		t.Fatalf("normalized name = %q", got.(*NameExpr).Name())
	}
	if gotLocation, found := store.Find(got.ID()); !found || gotLocation != location {
		t.Fatalf("normalized source = %v, %v", gotLocation, found)
	}
}
