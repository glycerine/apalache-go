package apalache

import "testing"

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
