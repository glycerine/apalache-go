package apalache

import "testing"

func TestTypeClauseScalaValueAndFluentCallbackContracts(t *testing.T) {
	a := mustNumberedVar(t, 0)
	equation := &EqClause{Unknown: a, Term: IntT1}
	disjunction := &OrClause{Clauses: []*EqClause{equation, {Unknown: a, Term: BoolT1}}}

	for _, clause := range []TypeClause{equation, disjunction} {
		foundCalls := 0
		errorCalls := 0
		returned := clause.SetOnTypeFound(func(typ Type) {
			if !EqualTypes(typ, IntT1) {
				t.Errorf("found callback type = %v", typ)
			}
			foundCalls++
		})
		if returned != clause {
			t.Fatal("SetOnTypeFound did not return the same clause")
		}
		returned = returned.SetOnTypeError(func(substitution *Substitution, terms []Type) {
			if substitution == nil || len(terms) != 2 {
				t.Errorf("error callback = %v, %v", substitution, terms)
			}
			errorCalls++
		})
		if returned != clause {
			t.Fatal("SetOnTypeError did not return the same clause")
		}
		found, onError := clause.callbacks()
		found(IntT1)
		onError(EmptySubstitution(), []Type{IntT1, BoolT1})
		if foundCalls != 1 || errorCalls != 1 {
			t.Fatalf("callback counts = %d, %d", foundCalls, errorCalls)
		}
	}

	if equation.Unknown != a || !EqualTypes(equation.Term, IntT1) || len(disjunction.Clauses) != 2 || disjunction.Clauses[0] != equation {
		t.Fatal("clause payload changed while installing callbacks")
	}
}

func TestEtcRefScalaValueContracts(t *testing.T) {
	id := NewUID()
	exact := ExactRef{SourceID: id}
	blame := BlameRef{SourceID: id}
	var exactInterface EtcRef = exact
	var blameInterface EtcRef = blame

	if exact.ID() != id || blame.ID() != id || exactInterface.ID() != blameInterface.ID() {
		t.Fatal("ETC references did not preserve the source UID")
	}
	if exact.String() != "E"+id.String() || blame.String() != "B"+id.String() {
		t.Fatalf("ETC reference strings = %q, %q", exact.String(), blame.String())
	}
	if exact == (ExactRef{SourceID: NewUID()}) || blame == (BlameRef{SourceID: NewUID()}) {
		t.Fatal("distinct ETC source references compared equal")
	}
	if exact.String() == blame.String() {
		t.Fatal("exact and blame references have indistinguishable displays")
	}
	if got := (ExactRef{SourceID: NullUID()}).String(); got != "E0" {
		t.Fatalf("null exact reference = %q", got)
	}
}
