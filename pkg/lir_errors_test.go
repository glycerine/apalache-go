package apalache

import (
	"errors"
	"testing"
)

func TestLirExceptionHierarchyAndPayloadsMatchScala(t *testing.T) {
	expression := NewNameExpr("bad", Untyped())
	module := NewModule("M", nil)
	failures := []LanguageFailure{{ID: expression.ID(), Message: "unsupported"}}

	language := NewLanguagePredError("language", failures)
	failures[0].Message = "mutated source"
	copyOfFailures := language.FailedIDs()
	copyOfFailures[0].Message = "mutated result"
	if language.Error() != "language" || language.FailedIDs()[0].Message != "unsupported" {
		t.Fatalf("language error = %q, %v", language.Error(), language.FailedIDs())
	}

	malformed := NewMalformedTlaError("malformed", expression)
	missing := NewMissingTransformationError("missing", module)
	builder := NewBuilderError("builder")
	cyclic := NewCyclicDependencyError("cyclic")
	outdated := NewOutdatedAnnotationsError("outdated", expression)
	for _, err := range []error{language, malformed, missing, builder, cyclic, outdated} {
		var lir *LirError
		if !errors.As(err, &lir) || lir.Error() != err.Error() {
			t.Errorf("%T does not unwrap to matching LirError", err)
		}
	}
	if malformed.CauseExpr() != expression || outdated.CauseExpr() != expression || missing.CauseModule() != module {
		t.Fatal("LIR exception cause payload was not retained")
	}
}

func TestNonLirExceptionsAndTypingPayloadMatchScala(t *testing.T) {
	typing := NewTypingException("typing", NullUID())
	input := NewTypingInputException("input", NewUID())
	malformed := NewMalformedSepecificationError("specification")
	unsupported := NewUnsupportedFeatureError("unsupported")
	for _, err := range []error{typing, malformed, unsupported} {
		var lir *LirError
		if errors.As(err, &lir) {
			t.Errorf("%T unexpectedly belongs to the LirError hierarchy", err)
		}
	}
	if typing.Error() != "typing" || typing.CauseExprID() != NullUID() ||
		input.Error() != "input" || input.CauseExprID() == NullUID() ||
		malformed.Error() != "specification" || unsupported.Error() != "unsupported" {
		t.Fatal("plain exception message or typing UID payload was not retained")
	}
	var typingBase *TypingException
	if !errors.As(input, &typingBase) || typingBase.CauseExprID() != input.CauseExprID() {
		t.Fatal("TypingInputException does not unwrap to TypingException")
	}
}

func TestLanguagePredicateWatcherUnwrapsToScalaError(t *testing.T) {
	expression := NewNameExpr("x", Untyped())
	err := &LanguagePredicateError{
		Subject:  "expressions",
		Failures: []LanguageFailure{{ID: expression.ID(), Message: "bad expression"}},
	}
	var scalaError *LanguagePredError
	if !errors.As(err, &scalaError) {
		t.Fatal("watcher error does not unwrap to LanguagePredError")
	}
	if scalaError.Error() != err.Error() || len(scalaError.FailedIDs()) != 1 ||
		scalaError.FailedIDs()[0].ID != expression.ID() {
		t.Fatalf("unwrapped watcher error = %q, %v", scalaError.Error(), scalaError.FailedIDs())
	}
}
