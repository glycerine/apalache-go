package apalache

import "testing"

func representativeOperatorArity(operator Operator) (int, bool) {
	for count := 0; count <= 12; count++ {
		if operator.IsCorrectArity(count) {
			return count, true
		}
	}
	return 0, false
}

func TestLevelOrderingAndNames(t *testing.T) {
	levels := []Level{ConstantLevel, StateLevel, ActionLevel, TemporalLevel}
	names := []string{"Constant", "State", "Action", "Temporal"}
	for index, level := range levels {
		if level.Number() != index || level.String() != names[index] {
			t.Fatalf("level %d = %d, %q", index, level.Number(), level.String())
		}
		if got, err := LevelFromInt(index); err != nil || got != level {
			t.Fatalf("LevelFromInt(%d) = %v, %v", index, got, err)
		}
	}
	if ConstantLevel.Join(ActionLevel) != ActionLevel || TemporalLevel.Join(StateLevel) != TemporalLevel {
		t.Fatal("level join is not maximum")
	}
	if _, err := LevelFromInt(4); err == nil {
		t.Fatal("invalid level accepted")
	}
}

func TestLevelFinderNamesAndOperators(t *testing.T) {
	constant := NewConstDecl("N", Untyped())
	variable := NewVarDecl("x", Untyped())
	constantDef := NewOperDecl("One", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())
	stateDef := NewOperDecl("Current", nil, NewNameExpr("x", Untyped()), Untyped())
	actionBody := MustOperExpr(Op("EQ"), Untyped(),
		MustOperExpr(Op("PRIME"), Untyped(), NewNameExpr("x", Untyped())), NewNameExpr("x", Untyped()))
	actionDef := NewOperDecl("Step", nil, actionBody, Untyped())
	temporalDef := NewOperDecl("Spec", nil,
		MustOperExpr(Op("GLOBALLY"), Untyped(), NewNameExpr("Step", Untyped())), Untyped())
	module := NewModule("Levels", []Decl{constant, variable, constantDef, stateDef, actionDef, temporalDef})
	finder := NewLevelFinder(module)

	tests := map[string]Level{
		"N": ConstantLevel, "x": StateLevel, "One": ConstantLevel,
		"Current": StateLevel, "Step": ActionLevel, "Spec": TemporalLevel,
		"unknown": ConstantLevel,
	}
	for name, want := range tests {
		if got := finder.LevelOfName(name); got != want {
			t.Errorf("LevelOfName(%q) = %v, want %v", name, got, want)
		}
	}
	if finder.LevelOfExpression(actionBody) != ActionLevel {
		t.Fatal("PRIME expression is not action-level")
	}
}

func TestLevelFinderLetAndRecursiveDefinitions(t *testing.T) {
	variable := NewVarDecl("x", Untyped())
	local := NewOperDecl("Local", nil,
		MustOperExpr(Op("PRIME"), Untyped(), NewNameExpr("x", Untyped())), Untyped())
	let := NewLetInExpr(NewNameExpr("Local", Untyped()), Untyped(), local)
	recursive := NewOperDecl("Recursive", nil,
		MustOperExpr(Op("OR"), Untyped(), NewNameExpr("Recursive", Untyped()), NewNameExpr("x", Untyped())), Untyped())
	module := NewModule("Recursive", []Decl{variable, recursive})
	finder := NewLevelFinder(module)
	if finder.LevelOfExpression(let) != ActionLevel {
		t.Fatal("LET-IN did not join the local declaration body level")
	}
	if finder.LevelOfDeclaration(recursive) != StateLevel {
		t.Fatal("recursive operator did not terminate at and preserve state level")
	}
}

func TestDeclarationLevelCompatibility(t *testing.T) {
	body := MustOperExpr(Op("GLOBALLY"), Untyped(), NewValueExpr(BoolValue(true), Untyped()))
	constant := NewConstDecl("C", Untyped())
	variable := NewVarDecl("v", Untyped())
	assumption := NewUnnamedAssumeDecl(body, Untyped())
	theorem := NewTheoremDecl("Thm", body, Untyped())
	operator := NewOperDecl("Temporal", nil, body, Untyped())
	finder := NewLevelFinder(NewModule("M", []Decl{constant, variable, assumption, theorem, operator}))

	if finder.LevelOfDeclarationWithoutCache(constant) != ConstantLevel ||
		finder.LevelOfDeclarationWithoutCache(variable) != StateLevel ||
		finder.LevelOfDeclarationWithoutCache(assumption) != ConstantLevel ||
		finder.LevelOfDeclarationWithoutCache(theorem) != ConstantLevel ||
		finder.LevelOfDeclarationWithoutCache(operator) != TemporalLevel {
		t.Fatal("declaration levels differ from Scala behavior")
	}
}

func TestLevelFinderExhaustiveScalaOperatorFamilies(t *testing.T) {
	variable := NewVarDecl("x", Untyped())
	finder := NewLevelFinder(NewModule("M", []Decl{variable}))
	for _, operator := range AllOperators() {
		wantBase := ConstantLevel
		switch operator.Category() {
		case CategoryAction:
			wantBase = ActionLevel
		case CategoryTemporal:
			wantBase = TemporalLevel
		}
		if got := operatorLevel(operator); got != wantBase {
			t.Errorf("operatorLevel(%s) = %v, want %v", operator, got, wantBase)
		}

		arity, ok := representativeOperatorArity(operator)
		if !ok || !operator.Constructible() {
			continue
		}
		constantArguments := make([]Expr, arity)
		stateArguments := make([]Expr, arity)
		for index := 0; index < arity; index++ {
			constantArguments[index] = NewValueExpr(Int64Value(int64(index)), Untyped())
			stateArguments[index] = NewNameExpr("x", Untyped())
		}
		if operator.Name() == "LABEL" {
			for index := 1; index < arity; index++ {
				constantArguments[index] = NewValueExpr(StringValue("label"), Untyped())
				stateArguments[index] = NewValueExpr(StringValue("label"), Untyped())
			}
		}
		constantExpression, err := NewOperExpr(operator, Untyped(), constantArguments...)
		if err != nil {
			t.Fatalf("construct %s/%d: %v", operator, arity, err)
		}
		if got := finder.LevelOfExpression(constantExpression); got != wantBase {
			t.Errorf("constant %s expression = %v, want %v", operator, got, wantBase)
		}
		stateExpression, err := NewOperExpr(operator, Untyped(), stateArguments...)
		if err != nil {
			t.Fatalf("construct state %s/%d: %v", operator, arity, err)
		}
		wantState := wantBase
		if arity > 0 {
			wantState = wantState.Join(StateLevel)
		}
		if got := finder.LevelOfExpression(stateExpression); got != wantState {
			t.Errorf("state %s expression = %v, want %v", operator, got, wantState)
		}
	}
}

func TestLevelFinderDeclarationAndCacheScalaCases(t *testing.T) {
	variable := NewVarDecl("x", Untyped())
	state := NewOperDecl("State", nil, NewNameExpr("x", Untyped()), Untyped())
	action := NewOperDecl("Action", nil,
		MustOperExpr(Op("PRIME"), Untyped(), NewNameExpr("State", Untyped())), Untyped())
	temporal := NewOperDecl("Temporal", nil,
		MustOperExpr(Op("GLOBALLY"), Untyped(), NewNameExpr("Action", Untyped())), Untyped())
	left := NewOperDecl("Left", nil, NewNameExpr("Right", Untyped()), Untyped())
	right := NewOperDecl("Right", nil,
		MustOperExpr(Op("OR"), Untyped(), NewNameExpr("Left", Untyped()), NewNameExpr("x", Untyped())), Untyped())
	module := NewModule("M", []Decl{variable, state, action, temporal, left, right})
	finder := NewLevelFinder(module)
	for _, test := range []struct {
		declaration *OperDecl
		want        Level
	}{
		{state, StateLevel},
		{action, ActionLevel},
		{temporal, TemporalLevel},
		{left, StateLevel},
		{right, StateLevel},
	} {
		if got := finder.LevelOfDeclaration(test.declaration); got != test.want {
			t.Errorf("LevelOfDeclaration(%s) = %v, want %v", test.declaration.Name(), got, test.want)
		}
		if got := finder.LevelOfName(test.declaration.Name()); got != test.want {
			t.Errorf("cached LevelOfName(%s) = %v, want %v", test.declaration.Name(), got, test.want)
		}
	}

	localState := NewOperDecl("LocalState", nil, NewNameExpr("x", Untyped()), Untyped())
	localTemporal := NewOperDecl("LocalTemporal", nil,
		MustOperExpr(Op("EVENTUALLY"), Untyped(), NewValueExpr(BoolValue(true), Untyped())), Untyped())
	let := NewLetInExpr(NewValueExpr(Int64Value(0), Untyped()), Untyped(), localState, localTemporal)
	if got := finder.LevelOfExpression(let); got != TemporalLevel {
		t.Fatalf("LET body/declaration join = %v, want Temporal", got)
	}
}
