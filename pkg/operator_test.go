package apalache

import "testing"

func TestArityCombinatorsMatchScala(t *testing.T) {
	tests := []struct {
		name   string
		arity  Arity
		accept []int
		reject []int
	}{
		{"fixed", FixedArity(2), []int{2}, []int{-1, 0, 1, 3}},
		{"minimum", MinimalArity(2), []int{2, 3, 10}, []int{-1, 0, 1}},
		{"any", AnyArity(), []int{0, 1, 10}, []int{-1}},
		{"odd", AnyOddArity(), []int{1, 3}, []int{-1, 0, 2}},
		{"even", AnyEvenArity(), []int{0, 2}, []int{-2, -1, 1}},
		{"positive", AnyPositiveArity(), []int{1, 2}, []int{-1, 0}},
		{"and", AnyOddArity().And(MinimalArity(3)), []int{3, 5}, []int{-1, 0, 1, 2, 4}},
		{"or", FixedArity(2).Or(FixedArity(3)), []int{2, 3}, []int{-1, 0, 1, 4}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, count := range test.accept {
				if !test.arity.Accepts(count) {
					t.Errorf("Accepts(%d) = false", count)
				}
			}
			for _, count := range test.reject {
				if test.arity.Accepts(count) {
					t.Errorf("Accepts(%d) = true", count)
				}
			}
		})
	}
}

func TestOperatorRegistry(t *testing.T) {
	operators := AllOperators()
	if len(operators) != 105 {
		t.Fatalf("registered %d operators, want 105", len(operators))
	}
	seen := make(map[string]bool)
	for index, operator := range operators {
		if seen[operator.Name()] {
			t.Fatalf("duplicate operator %q", operator.Name())
		}
		seen[operator.Name()] = true
		if index > 0 && operators[index-1].Name() >= operator.Name() {
			t.Fatalf("operators are not strictly sorted at %q, %q", operators[index-1].Name(), operator.Name())
		}
	}
	if operators[0].Name() == "" {
		t.Fatal("empty operator name")
	}
}

func TestRepresentativeOperatorMetadata(t *testing.T) {
	tests := []struct {
		name           string
		category       OperatorCategory
		interpretation Interpretation
		accept, reject int
		precedence     Precedence
	}{
		{"EQ", CategoryBase, InterpretationPredefined, 2, 1, Precedence{50, 50}},
		{"AND", CategoryBoolean, InterpretationPredefined, 0, -1, Precedence{30, 30}},
		{"PLUS", CategoryArithmetic, InterpretationStandardLibrary, 2, 1, Precedence{100, 100}},
		{"SET_MAP", CategorySet, InterpretationPredefined, 5, 4, Precedence{160, 160}},
		{"EXCEPT", CategoryFunction, InterpretationPredefined, 3, 2, Precedence{160, 160}},
		{"GLOBALLY", CategoryTemporal, InterpretationPredefined, 1, 2, Precedence{40, 150}},
		{"Apalache!Guess", CategoryApalache, InterpretationPredefined, 1, 2, Precedence{160, 160}},
		{"SequencesExt!ToSet", CategorySequence, InterpretationStandardLibrary, 1, 2, Precedence{160, 160}},
		{"Apalache!StoreInSet", CategoryApalacheInternal, InterpretationStandardLibrary, 3, 4, Precedence{160, 160}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operator, ok := LookupOperator(test.name)
			if !ok {
				t.Fatal("operator not found")
			}
			if operator.Category() != test.category || operator.Interpretation() != test.interpretation || operator.Precedence() != test.precedence {
				t.Fatalf("metadata = category %s interpretation %v precedence %v", operator.Category(), operator.Interpretation(), operator.Precedence())
			}
			if !operator.IsCorrectArity(test.accept) || operator.IsCorrectArity(test.reject) {
				t.Fatalf("arity accepts %d=%v rejects %d=%v", test.accept, operator.IsCorrectArity(test.accept), test.reject, operator.IsCorrectArity(test.reject))
			}
		})
	}
}

func TestProhibitedAndParameterizedOperators(t *testing.T) {
	if Op("Sequences!SelectSeq").Constructible() || Op("Apalache!FunAsSeq").Constructible() {
		t.Fatal("rewired Scala operators are constructible")
	}
	mapped := NewSMTMapOperator(Op("AND"))
	if mapped.Name() != "Apalache!SmtMap_AND" || !mapped.IsCorrectArity(2) || mapped.IsCorrectArity(1) {
		t.Fatalf("SMT map metadata = %#v", mapped)
	}
}
