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
	if _, err := OffsetRenamedName(map[string]int{"x": 1}, "x$1"); err == nil {
		t.Fatal("accepted a non-positive shifted counter")
	}
	if shifted, err := OffsetRenamedName(map[string]int{"x": 4}, "y$2"); err != nil || shifted != "y$2" {
		t.Fatalf("unmentioned base shifted to %q, %v", shifted, err)
	}
}

func TestIncrementalRenamingEveryScalaBinderOperator(t *testing.T) {
	for _, operator := range []string{"SET_FILTER", "EXISTS3", "FORALL3", "CHOOSE3", "CHOOSE2"} {
		x := NewNameExpr("x", Untyped())
		arity := 3
		if operator == "CHOOSE2" {
			arity = 2
		}
		arguments := make([]Expr, arity)
		for index := range arguments {
			arguments[index] = x
		}
		got := NewIncrementalRenamer(IdleTracker{}).Rename(MustOperExpr(Op(operator), Untyped(), arguments...)).(*OperExpr)
		for index, argument := range got.Arguments() {
			if name := argument.(*NameExpr).Name(); name != "x$1" {
				t.Errorf("%s argument %d = %s, want x$1", operator, index, name)
			}
		}
	}
	for _, operator := range []string{"SET_MAP", "FUN_CTOR", "FUN_REC_CTOR"} {
		x := NewNameExpr("x", Untyped())
		set := NewNameExpr("S", Untyped())
		got := NewIncrementalRenamer(IdleTracker{}).Rename(
			MustOperExpr(Op(operator), Untyped(), x, x, set),
		).(*OperExpr).Arguments()
		if got[0].(*NameExpr).Name() != "x$1" || got[1].(*NameExpr).Name() != "x$1" {
			t.Errorf("%s did not rename result and binder together", operator)
		}
	}
}

func TestNameCounterMapsMatchScalaOracle(t *testing.T) {
	plain := MustOperExpr(Op("AND"), Untyped(), NewNameExpr("x", Untyped()), NewNameExpr("y", Untyped()))
	if got := NameCountersInExpression(plain, true); len(got) != 0 {
		t.Fatalf("plain counters = %v", got)
	}
	expression := MustOperExpr(Op("AND"), Untyped(),
		NewNameExpr("x$2", Untyped()), NewNameExpr("y$8", Untyped()),
		MustOperExpr(Op("EQ"), Untyped(),
			MustOperExpr(Op("PLUS"), Untyped(), NewNameExpr("x$1", Untyped()),
				MustOperExpr(Op("PRIME"), Untyped(), NewNameExpr("z$3", Untyped()))),
			NewValueExpr(Int64Value(2), Untyped())))
	if got, want := NameCountersInExpression(expression, true), map[string]int{"x": 2, "y": 8, "z": 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("maximum counters = %v, want %v", got, want)
	}
	if got, want := NameCountersInExpression(expression, false), map[string]int{"x": 1, "y": 8, "z": 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("minimum counters = %v, want %v", got, want)
	}

	local := NewOperDecl("Y$6", []OperatorParam{{Name: "p$3"}, {Name: "t$99", Arity: 2}},
		MustOperExpr(Op("PLUS"), Untyped(), NewNameExpr("p$3", Untyped()), NewNameExpr("s$5", Untyped())), Untyped())
	let := NewLetInExpr(NewNameExpr("Y$6", Untyped()), Untyped(), local)
	if got, want := NameCountersInExpression(let, true), map[string]int{"Y": 6, "p": 3, "t": 99, "s": 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("LET counters = %v, want %v", got, want)
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

func TestIncrementalRenamingBatchAPIsMatchScala(t *testing.T) {
	s := NewNameExpr("S", Untyped())
	p := NewNameExpr("p", Untyped())
	quantifier := func(name string) Expr {
		return MustOperExpr(Op("EXISTS3"), Untyped(), NewNameExpr(name, Untyped()), s, p)
	}
	renamer := NewIncrementalRenamer(IdleTracker{})
	expressions, err := renamer.NormalizeExpressions([]Expr{quantifier("x$3"), quantifier("x$105")})
	if err != nil {
		t.Fatal(err)
	}
	if first := expressions[0].(*OperExpr).Arguments()[0].(*NameExpr).Name(); first != "x$1" {
		t.Fatalf("first normalized expression binder = %s", first)
	}
	if second := expressions[1].(*OperExpr).Arguments()[0].(*NameExpr).Name(); second != "x$103" {
		t.Fatalf("second normalized expression binder = %s", second)
	}

	renamer.SyncFrom(quantifier("x$99"))
	declarations := []Decl{
		NewOperDecl("X", nil, quantifier("x$3"), Untyped()),
		NewOperDecl("Y", nil, quantifier("x$105"), Untyped()),
		NewVarDecl("untouched", Untyped()),
	}
	result, err := renamer.SyncAndNormalizeDeclarations(declarations)
	if err != nil {
		t.Fatal(err)
	}
	if first := result[0].(*OperDecl).Body().(*OperExpr).Arguments()[0].(*NameExpr).Name(); first != "x$1" {
		t.Fatalf("first declaration binder = %s", first)
	}
	if second := result[1].(*OperDecl).Body().(*OperExpr).Arguments()[0].(*NameExpr).Name(); second != "x$2" {
		t.Fatalf("second declaration binder = %s", second)
	}
	if result[2] != declarations[2] {
		t.Fatal("non-operator declaration was not preserved")
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

func TestIncrementalRenamingNoDuplicateDefinitionsScalaProperty(t *testing.T) {
	for caseNo := 0; caseNo < 2500; caseNo++ {
		x := NewNameExpr("x", Untyped())
		y := NewNameExpr("y", Untyped())
		set := NewNameExpr("S", Untyped())
		truth := NewValueExpr(BoolValue(true), Untyped())
		left := MustOperExpr(Op("EXISTS3"), Untyped(), x, set,
			MustOperExpr(Op("FORALL3"), Untyped(), x, set, truth))
		right := MustOperExpr(Op("SET_FILTER"), Untyped(), x, set, truth)
		mapped := MustOperExpr(Op("SET_MAP"), Untyped(), x, x, set, y, set)
		local := NewOperDecl("Local", []OperatorParam{ValueParam("x")},
			MustOperExpr(Op("CHOOSE2"), Untyped(), x, x), Untyped())
		let := NewLetInExpr(untypedApp("Local", x), Untyped(), local)
		parts := []Expr{left, right, mapped, let}
		body := parts[caseNo%len(parts)]
		for depth := 0; depth < caseNo%5; depth++ {
			body = MustOperExpr(Op("AND"), Untyped(), body, parts[(caseNo+depth+1)%len(parts)])
		}
		module := NewModule("Generated", []Decl{
			NewOperDecl("A", nil, body, Untyped()),
			NewOperDecl("B", nil, parts[(caseNo+2)%len(parts)], Untyped()),
		})
		renamed, err := NewIncrementalRenamer(IdleTracker{}).RenameModule(module)
		if err != nil {
			t.Fatalf("case %d: %v", caseNo, err)
		}
		counts := make(map[string]int)
		for _, declaration := range renamed.OperDeclarations() {
			collectRenamedDefinitions(declaration.Body(), counts)
		}
		for name, count := range counts {
			if count > 1 {
				t.Fatalf("case %d: name %s defined %d times", caseNo, name, count)
			}
		}
	}
}

func collectRenamedDefinitions(expression Expr, counts map[string]int) {
	switch typed := expression.(type) {
	case *LetInExpr:
		collectRenamedDefinitions(typed.Body(), counts)
		for _, declaration := range typed.Declarations() {
			counts[declaration.Name()]++
			collectRenamedDefinitions(declaration.Body(), counts)
		}
	case *OperExpr:
		arguments := typed.Arguments()
		if isSingleBinder(typed.Operator().Name()) && len(arguments) > 0 {
			if name, ok := arguments[0].(*NameExpr); ok {
				counts[name.Name()]++
			}
		} else if isMultipleBinder(typed.Operator().Name()) {
			for index := 1; index < len(arguments); index += 2 {
				if name, ok := arguments[index].(*NameExpr); ok {
					counts[name.Name()]++
				}
			}
		}
		for _, argument := range arguments {
			collectRenamedDefinitions(argument, counts)
		}
	}
}
