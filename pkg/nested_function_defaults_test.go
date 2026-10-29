package apalache

import (
	"context"
	"fmt"
	"testing"
)

func TestNestedFunctionDefaults(t *testing.T) {
	for _, text := range []string{`<<(Int -> Int), Int>>`, `{f: (Int -> Int)}`, `F((Int -> Int)) | N(Int)`, `Seq((Int -> Int))`} {
		t.Run(text, func(t *testing.T) {
			typ, err := ParseType(text)
			if err != nil {
				t.Fatal(err)
			}
			rewriter := NewStratifiedRewriter()
			scope, cell, err := rewriter.DefaultValueCache().GetOrCreate(InitialRewriterScope(), typ)
			if err != nil {
				t.Fatal(err)
			}
			if !EqualTypes(cell.Type.TLAType(), typ) {
				t.Fatalf("wrong default type: %s", cell.Type)
			}
			sameScope, same, err := rewriter.DefaultValueCache().GetOrCreate(scope, typ)
			if err != nil || !same.Equal(cell) || sameScope.Arena != scope.Arena {
				t.Fatal("default not cached", err)
			}
		})
	}
}

func TestSubSeqAndConcatWithNestedFunctions(t *testing.T) {
	for _, inv := range []string{
		`SubSeq(<< <<[i \in {1}|->i],2>> >>,1,x)[1][2]=2`,
		`(SubSeq(<< <<[i \in {1}|->i],2>> >>,1,x) \o << <<[i \in {1}|->i],2>> >>)[2][2]=2`,
	} {
		prepared := prepareSafetyModelFixture(t, fmt.Sprintf("---- MODULE Review ----\nEXTENDS Integers, Sequences\nVARIABLE\n\\* @type: Int;\nx\nInit == x=2\nNext == x'=x\nInv == %s\n====", inv), []string{"Inv"}, "")
		result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates, BoundedSafetyOptions{MaxSteps: 0})
		if err != nil || result.Outcome != SafetyNoCounterexample {
			t.Fatalf("%s: %v, %v", inv, result.Outcome, err)
		}
	}
}
