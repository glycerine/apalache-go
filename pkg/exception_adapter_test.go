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
