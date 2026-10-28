package apalache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ModelCheckerListener observes traces produced by bounded model checking.
// Indices are the checker search counters used to name trace files; invariant
// errors are one-based, while a deadlock is reported with error index zero.
type ModelCheckerListener interface {
	OnCounterexample(counterexample BoundedCounterexample, errorIndex int) error
	OnExample(example []DecodedState, exampleIndex int) error
}

// CollectCounterexamplesModelCheckerListener retains counterexamples and
// ignores example traces, matching the Scala test/oracle listener.
type CollectCounterexamplesModelCheckerListener struct {
	mu              sync.Mutex
	counterexamples []BoundedCounterexample
}

func (listener *CollectCounterexamplesModelCheckerListener) OnCounterexample(
	counterexample BoundedCounterexample,
	_ int,
) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()
	listener.counterexamples = append(listener.counterexamples, cloneBoundedCounterexample(counterexample))
	return nil
}

func (*CollectCounterexamplesModelCheckerListener) OnExample([]DecodedState, int) error {
	return nil
}

// Counterexamples returns a defensive snapshot in callback order.
func (listener *CollectCounterexamplesModelCheckerListener) Counterexamples() []BoundedCounterexample {
	listener.mu.Lock()
	defer listener.mu.Unlock()
	result := make([]BoundedCounterexample, len(listener.counterexamples))
	for index, counterexample := range listener.counterexamples {
		result[index] = cloneBoundedCounterexample(counterexample)
	}
	return result
}

// DumpFilesModelCheckerListener writes both the latest trace and an indexed
// trace in every standalone output format.
type DumpFilesModelCheckerListener struct {
	Module    *Module
	Directory string
}

func NewDumpFilesModelCheckerListener(module *Module, directory string) *DumpFilesModelCheckerListener {
	return &DumpFilesModelCheckerListener{Module: module, Directory: directory}
}

func (listener *DumpFilesModelCheckerListener) OnCounterexample(
	counterexample BoundedCounterexample,
	errorIndex int,
) error {
	violation := traceViolationForCounterexample(listener.Module, counterexample)
	return listener.dump(counterexample.States, violation, "violation", errorIndex)
}

func (listener *DumpFilesModelCheckerListener) OnExample(example []DecodedState, exampleIndex int) error {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	return listener.dump(example, truth, "example", exampleIndex)
}

func (listener *DumpFilesModelCheckerListener) dump(
	states []DecodedState,
	violation Expr,
	prefix string,
	index int,
) error {
	if listener == nil || listener.Module == nil {
		return fmt.Errorf("cannot dump a model-checker trace without a module")
	}
	if index < 0 {
		return fmt.Errorf("model-checker trace index must be non-negative, found %d", index)
	}
	directory := listener.Directory
	if directory == "" {
		directory = "."
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create trace directory %s: %w", directory, err)
	}
	if _, err := writeDecodedTraceFiles(directory, prefix, listener.Module, states, violation); err != nil {
		return err
	}
	_, err := writeDecodedTraceFiles(directory, fmt.Sprintf("%s%d", prefix, index), listener.Module, states, violation)
	return err
}

func writeDecodedTraceFiles(
	directory string,
	prefix string,
	module *Module,
	states []DecodedState,
	violation Expr,
) ([]string, error) {
	tla, err := FormatDecodedTraceTLA(module, states, violation)
	if err != nil {
		return nil, err
	}
	tlc, err := FormatDecodedTraceTLC(module, states)
	if err != nil {
		return nil, err
	}
	legacy, err := MarshalDecodedTraceApalacheJSON(module, states, violation)
	if err != nil {
		return nil, err
	}
	itf, err := MarshalDecodedTraceITF(module, states, ITFOptions{})
	if err != nil {
		return nil, err
	}
	files := []struct {
		name string
		data []byte
	}{
		{prefix + ".tla", []byte(tla)},
		{"MC" + prefix + ".out", []byte(tlc)},
		{prefix + ".json", append(legacy, '\n')},
		{prefix + ".itf.json", append(itf, '\n')},
	}
	written := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(directory, file.name)
		if err = os.WriteFile(path, file.data, 0o644); err != nil {
			return written, fmt.Errorf("write trace file %s: %w", path, err)
		}
		written = append(written, path)
	}
	return written, nil
}

func traceViolationForCounterexample(module *Module, counterexample BoundedCounterexample) Expr {
	if module != nil && counterexample.Invariant != "" {
		if declaration, found := declarationNamed(module, counterexample.Invariant); found {
			if operator, ok := declaration.(*OperDecl); ok {
				return MustOperExpr(Op("NOT"), Typed(BoolT1), operator.Body())
			}
		}
	}
	return NewValueExpr(BoolValue(true), Typed(BoolT1))
}

func cloneBoundedCounterexample(counterexample BoundedCounterexample) BoundedCounterexample {
	result := counterexample
	result.NextTransitions = append([]int(nil), counterexample.NextTransitions...)
	result.States = cloneDecodedStates(counterexample.States)
	result.View = append([]Expr(nil), counterexample.View...)
	return result
}

func cloneDecodedStates(states []DecodedState) []DecodedState {
	result := make([]DecodedState, len(states))
	for index, state := range states {
		values := make(map[string]Expr, len(state.Values))
		for name, value := range state.Values {
			values[name] = value
		}
		result[index] = DecodedState{Values: values, TransitionNumber: state.TransitionNumber}
	}
	return result
}
