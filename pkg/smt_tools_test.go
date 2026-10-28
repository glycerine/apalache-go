package apalache

import "testing"

func TestSMTToolsFormattingMatchesScala(t *testing.T) {
	tests := []struct {
		formula SMTBoolFormula
		want    string
	}{
		{SMTFalse{}, "false"},
		{NewSMTAnd(), "( and  )"},
		{NewSMTAnd(SMTVariable{ID: 1}, SMTFalse{}), "( and b_1 false )"},
		{NewSMTOr(SMTVariable{ID: -2}), "( or b_-2 )"},
		{SMTNeg{Argument: SMTVariable{ID: 3}}, "( not b_3 )"},
		{SMTImplies{Left: SMTVariable{ID: 1}, Right: SMTVariable{ID: 2}}, "( => b_1 b_2 )"},
		{SMTLtFunctions{Left: 1, Right: 2}, "( < ( R 1 ) ( R 2 ) )"},
		{SMTNeFunctions{Left: -1, Right: 4}, "( not ( = ( R -1 ) ( R 4 ) ) )"},
	}
	for _, test := range tests {
		if got := FormatSMTBoolFormula(test.formula); got != test.want {
			t.Errorf("FormatSMTBoolFormula(%T) = %q, want %q", test.formula, got, test.want)
		}
	}

	custom := NewSMTAnd(
		SMTVariable{ID: 1}, SMTLtFunctions{Left: 2, Right: 3},
		SMTNeg{Argument: SMTVariable{ID: 4}},
		SMTImplies{Left: SMTVariable{ID: 5}, Right: SMTVariable{ID: 6}},
	)
	want := "( and x_1 ( < ( F 2 ) ( F 3 ) ) ( not b_4 ) ( => b_5 b_6 ) )"
	if got := FormatSMTBoolFormulaSymbols(custom, "x", "F"); got != want {
		t.Fatalf("custom-symbol formatting = %q, want %q", got, want)
	}
}

func TestSMTToolsSimplificationMatchesScala(t *testing.T) {
	a, b := SMTVariable{ID: 1}, SMTVariable{ID: 2}
	tests := []struct {
		name  string
		input SMTBoolFormula
		want  string
	}{
		{"false", SMTFalse{}, "false"},
		{"and propagates nested false", NewSMTAnd(a, NewSMTOr(SMTFalse{})), "false"},
		{"and retains empty", NewSMTAnd(), "( and  )"},
		{"and retains singleton", NewSMTAnd(a), "( and b_1 )"},
		{"or drops false", NewSMTOr(SMTFalse{}, a, SMTFalse{}, b), "( or b_1 b_2 )"},
		{"or empty becomes false", NewSMTOr(), "false"},
		{"or singleton is pruned", NewSMTOr(SMTFalse{}, a), "b_1"},
		{"negation is not traversed", SMTNeg{Argument: NewSMTOr()}, "( not ( or  ) )"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatSMTBoolFormula(SimplifySMTBoolFormula(test.input)); got != test.want {
				t.Fatalf("simplified = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSMTToolsFormulaArgumentsAreDefensivelyCopied(t *testing.T) {
	arguments := []SMTBoolFormula{SMTVariable{ID: 1}}
	and := NewSMTAnd(arguments...)
	or := NewSMTOr(arguments...)
	arguments[0] = SMTFalse{}
	andCopy, orCopy := and.Arguments(), or.Arguments()
	andCopy[0], orCopy[0] = SMTFalse{}, SMTFalse{}
	if FormatSMTBoolFormula(and) != "( and b_1 )" || FormatSMTBoolFormula(or) != "( or b_1 )" {
		t.Fatal("formula arguments were mutated through source or accessor slices")
	}
}
