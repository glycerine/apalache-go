package apalache

import (
	"errors"
	"reflect"
	"testing"
)

func TestAssignmentExceptionExactMessageAndClassification(t *testing.T) {
	err := &AssignmentException{Message: "diagnostic"}
	if err.Error() != "diagnostic" {
		t.Fatalf("assignment exception = %q", err.Error())
	}
	var typed *AssignmentError
	if !errors.As(err, &typed) || typed != err {
		t.Fatalf("assignment exception classification = %T", err)
	}
}

func TestAssignmentPackageAliasesPreserveOrderedUIDStrategyAndExpression(t *testing.T) {
	first, second := NewUID(), NewUID()
	var variable Var = "x"
	var strategy StrategyType = []UID{first, second}
	var assignment AssignmentType = strategy
	expression := NewNameExpr(variable, Untyped())
	var transition SymbTrans = SymbolicTransition{Assignments: assignment, Expression: expression}
	if variable != "x" || !reflect.DeepEqual(transition.Assignments, AssignmentStrategy{first, second}) || transition.Expression != expression {
		t.Fatalf("package aliases changed symbolic transition: %#v", transition)
	}
}
