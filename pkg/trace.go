package apalache

import "sort"

// Trace is a model-checking example or counterexample. Labels are represented
// as sorted unique slices, the Go equivalent of Scala's SortedSet values.
type Trace[T any] struct {
	module *Module
	states []TraceState
	labels [][]string
	data   T
}

func NewTrace[T any](module *Module, states []TraceState, labels [][]string, data T) Trace[T] {
	return Trace[T]{
		module: cloneTraceModule(module),
		states: cloneTraceStates(states),
		labels: normalizeTraceLabels(labels),
		data:   data,
	}
}

func cloneTraceModule(module *Module) *Module {
	if module == nil {
		return nil
	}
	clone := NewModule(module.Name(), module.Declarations())
	clone.copyMetadataFrom(module)
	return clone
}

func cloneTraceStates(states []TraceState) []TraceState {
	result := make([]TraceState, len(states))
	for index, state := range states {
		result[index] = cloneTraceState(state)
	}
	return result
}

func normalizeTraceLabels(labels [][]string) [][]string {
	result := make([][]string, len(labels))
	for index, stateLabels := range labels {
		seen := make(map[string]struct{}, len(stateLabels))
		for _, label := range stateLabels {
			seen[label] = struct{}{}
		}
		result[index] = make([]string, 0, len(seen))
		for label := range seen {
			result[index] = append(result[index], label)
		}
		sort.Strings(result[index])
	}
	return result
}

func (trace Trace[T]) Module() *Module      { return cloneTraceModule(trace.module) }
func (trace Trace[T]) States() []TraceState { return cloneTraceStates(trace.states) }
func (trace Trace[T]) Labels() [][]string   { return normalizeTraceLabels(trace.labels) }
func (trace Trace[T]) Data() T              { return trace.data }
func (trace Trace[T]) StateCount() int      { return len(trace.states) }
func (trace Trace[T]) LabelStateCount() int { return len(trace.labels) }

// MarshalTraceITF is Scala Trace's JSON view: labels and attached data are not
// part of the ITF counterexample representation.
func MarshalTraceITF[T any](trace Trace[T], options ITFOptions) ([]byte, error) {
	return MarshalITFTrace(trace.module, trace.states, options)
}
