package apalache

import (
	"errors"
	"testing"
)

func testModulePass(name string, dependencies, transformations []ModuleProperty, execute func(*Module) (*Module, error)) ModulePass {
	return ModulePassFunc{PassName: name, Requires: dependencies, Adds: transformations, Transform: execute}
}

func successfulTestPass(module *Module) (*Module, error) {
	return NewModule("TestModule", nil), nil
}

func TestPassChainExecutorRunsCorrectlyOrderedChain(t *testing.T) {
	first := testModulePass("TestPass", nil, []ModuleProperty{PropertyInlined}, successfulTestPass)
	second := testModulePass("TestPass", []ModuleProperty{PropertyInlined}, nil, successfulTestPass)
	result, err := NewPassChainExecutor(first, second).Run()
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasProperty(PropertyInlined) || result.Name() != "TestModule" {
		t.Fatalf("ordered pass result = %s, inlined %v", result.Name(), result.HasProperty(PropertyInlined))
	}
}

func TestPassChainExecutorRejectsBadlyOrderedChain(t *testing.T) {
	first := testModulePass("TestPass", nil, nil, successfulTestPass)
	second := testModulePass("TestPass", []ModuleProperty{PropertyInlined}, nil, successfulTestPass)
	_, err := NewPassChainExecutor(first, second).Run()
	if err == nil || err.Error() != "TestPass cannot run for a module without the properties: Inlined" {
		t.Fatalf("badly ordered chain error = %v", err)
	}
}

func TestPassChainExecutorStopsOnPassFailure(t *testing.T) {
	want, err := NewPassFailure("TestPass", nil, ExitCodeError)
	if err != nil {
		t.Fatal(err)
	}
	first := testModulePass("TestPass", nil, nil, successfulTestPass)
	failed := testModulePass("TestPass", nil, nil, func(*Module) (*Module, error) { return nil, want })
	runAfterFailure := false
	last := testModulePass("LastPass", nil, nil, func(module *Module) (*Module, error) {
		runAfterFailure = true
		return module, nil
	})
	_, runErr := NewPassChainExecutor(first, failed, last).Run()
	var structured *PassFailure
	if !errors.As(runErr, &structured) || structured != want || runAfterFailure {
		t.Fatalf("failed chain = %v, later pass ran %v", runErr, runAfterFailure)
	}
}

func TestPassChainExecutorRunsOnlyOnce(t *testing.T) {
	executor := NewPassChainExecutor()
	if _, err := executor.Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Run(); err == nil {
		t.Fatal("pass-chain executor ran twice")
	}
}
