package apalache

import (
	"errors"
	"testing"
)

func TestErrorMessageClassification(t *testing.T) {
	normal := NormalErrorMessage{Text: "input error"}
	failure := FailureMessage{Text: "internal error"}
	if normal.Message() != "input error" || normal.RequiresStackTrace() {
		t.Fatalf("normal message = %#v", normal)
	}
	if failure.Message() != "internal error" || !failure.RequiresStackTrace() {
		t.Fatalf("failure message = %#v", failure)
	}
	adapted := &AdaptedError{Message: failure}
	if adapted.Error() != "internal error" {
		t.Fatalf("adapted error = %q", adapted.Error())
	}
}

func TestPassInfrastructureErrorTypes(t *testing.T) {
	execution := &PassExecError{Text: "execution failed"}
	option := &PassOptionError{Text: "invalid option"}
	if execution.Error() != "execution failed" || option.Error() != "invalid option" {
		t.Fatalf("pass errors = %q, %q", execution, option)
	}
}

func TestPassChainExecutorAdaptsRecognizedErrors(t *testing.T) {
	want := errors.New("known failure")
	adapter := ExceptionAdapterFunc(func(err error) (ErrorMessage, bool) {
		if errors.Is(err, want) {
			return NormalErrorMessage{Text: "friendly failure"}, true
		}
		return nil, false
	})
	failed := testModulePass("TestPass", nil, nil, func(*Module) (*Module, error) { return nil, want })
	_, err := NewPassChainExecutorWithAdapter(adapter, failed).Run()
	var adapted *AdaptedError
	if !errors.As(err, &adapted) || adapted.Error() != "friendly failure" || adapted.Message.RequiresStackTrace() {
		t.Fatalf("adapted pass error = %#v", err)
	}
}

func TestPassChainExecutorAdaptsDependencyErrors(t *testing.T) {
	adapter := ExceptionAdapterFunc(func(err error) (ErrorMessage, bool) {
		return FailureMessage{Text: "adapted: " + err.Error()}, true
	})
	pass := testModulePass("NeedsInline", []ModuleProperty{PropertyInlined}, nil, successfulTestPass)
	_, err := NewPassChainExecutorWithAdapter(adapter, pass).Run()
	var adapted *AdaptedError
	if !errors.As(err, &adapted) || adapted.Error() != "adapted: NeedsInline cannot run for a module without the properties: Inlined" || !adapted.Message.RequiresStackTrace() {
		t.Fatalf("adapted dependency error = %#v", err)
	}
}

func TestPassChainExecutorLeavesUnrecognizedErrorsAlone(t *testing.T) {
	want := errors.New("unrecognized")
	failed := testModulePass("TestPass", nil, nil, func(*Module) (*Module, error) { return nil, want })
	_, err := NewPassChainExecutorWithAdapter(DefaultExceptionAdapter{}, failed).Run()
	if !errors.Is(err, want) {
		t.Fatalf("unrecognized pass error = %v", err)
	}
}

func TestEtcTypeCheckerAdapterMatchesScalaClassification(t *testing.T) {
	store := NewSourceStore()
	expression := NewNameExpr("old", Untyped())
	location := SourceLocation{Filename: "Types", Region: NewSourceRegion(2, 3, 2, 9)}
	if err := store.Add(expression.ID(), location); err != nil {
		t.Fatal(err)
	}
	adapter := EtcTypeCheckerAdapter{Sources: store}
	tests := []struct {
		name       string
		err        error
		want       string
		stackTrace bool
	}{
		{"input", NewTypingInputException("bad annotation", expression.ID()), "Typing input error: bad annotation", false},
		{"outdated", NewOutdatedAnnotationsError("old syntax", expression), "Types.tla:2:3-2:9: rewriter error: old syntax", false},
		{"typing", NewTypingException("contradiction", expression.ID()), "Type checker error: contradiction", true},
		{"unknown source", NewOutdatedAnnotationsError("old syntax", NewNameExpr("missing", Untyped())), "<unknown>: rewriter error: old syntax", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message, matched := adapter.Adapt(test.err)
			if !matched || message.Message() != test.want || message.RequiresStackTrace() != test.stackTrace {
				t.Fatalf("Adapt(%T) = %#v, %v", test.err, message, matched)
			}
		})
	}
	if message, matched := adapter.Adapt(errors.New("other")); matched || message != nil {
		t.Fatalf("unrecognized error = %#v, %v", message, matched)
	}
}
