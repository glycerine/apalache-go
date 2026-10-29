package apalache

import "testing"

func TestDiagnosticErrorRetainsScalaSourceRange(t *testing.T) {
	diagnostic := Diagnostic{
		Code: "E-TYPE", Severity: SeverityError, Message: "bad operands",
		Position: Position{Filename: "Spec.tla", Line: 21, Column: 9, EndLine: 21, EndColumn: 26},
	}
	want := "Spec.tla:21:9-21:26: error E-TYPE: bad operands"
	if got := diagnostic.Error(); got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}
