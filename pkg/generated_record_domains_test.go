package apalache

import (
	"context"
	"testing"
)

func TestGeneratedLegacyRecordsHaveDomains(t *testing.T) {
	for _, text := range []string{`[a: Int]`, `<<[a: Int], Bool>>`, `[inner: [a: Int]]`} {
		t.Run(text, func(t *testing.T) {
			typ, err := ParseType(text)
			if err != nil {
				t.Fatal(err)
			}
			rewriter := NewStratifiedRewriter()
			expression := MustOperExpr(Op("Apalache!Gen"), Typed(typ), intLiteral(1))
			scope, cell, err := rewriter.Rewrite(expression, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			var check func(ArenaCell)
			check = func(c ArenaCell) {
				if record, ok := c.Type.TLAType().(RecordType); ok {
					domain, err := scope.Arena.Domain(c)
					if err != nil {
						t.Fatal(err)
					}
					if len(scope.Arena.GetHas(domain)) != len(record.Fields()) {
						t.Fatalf("incomplete domain for %s", record)
					}
					for _, p := range scope.Arena.GetHas(domain) {
						if _, ok := p.(FixedElementPointer); !ok {
							t.Fatalf("optional generated record key: %v", p)
						}
					}
				}
				for _, p := range scope.Arena.GetHas(c) {
					check(p.Element())
				}
			}
			check(cell)
		})
	}
}

func TestGeneratedLegacyRecordSupportsExceptAndEquality(t *testing.T) {
	prepared := prepareSafetyModelFixture(t, `---- MODULE Review ----
EXTENDS Integers, Apalache
VARIABLE
\* @type: Int;
x
Init == x=2
Next == x'=x
Inv == LET (* @type: [a: Int]; *) R == Gen(1) IN [R EXCEPT !.a=0]=[a|->0]
====`, []string{"Inv"}, "")
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates, BoundedSafetyOptions{MaxSteps: 0})
	if err != nil || result.Outcome != SafetyNoCounterexample {
		t.Fatalf("result=%v err=%v", result.Outcome, err)
	}
}
