package model

// Source: src/model/model_evaluator.cpp:250; src/model/model_core.cpp:29,81,105;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (f *FunctionInterpretation) traceEvaluatorFunctionAllocated(tags *diagnostic.TraceTags) {
	f.memoryTrace = tags
	f.memoryTraceLive = true
	f.traceFunctionAllocated("src/model/model_evaluator.cpp", 250)
}

// NewExtensionFunctionInterpretation owns the concrete function graph created
// by the reached nonconstant extension-model reconstruction branch.
func NewExtensionFunctionInterpretation(m *ast.ASTManager, arity uint32) (*FunctionInterpretation, error) {
	f, err := NewFunctionInterpretation(m, arity)
	if err != nil {
		return nil, err
	}
	f.memoryTrace = m.BorrowMemoryTraceTags()
	f.memoryTraceLive = true
	f.traceFunctionAllocated("src/tactic/extension_model_converter.cpp", 66)
	return f, nil
}

func (f *FunctionInterpretation) traceCopiedFunctionAllocated(tags *diagnostic.TraceTags) {
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	f.memoryTrace = tags
	f.memoryTraceLive = true
	f.traceFunctionAllocated("src/model/func_interp.cpp", 91)
}

func (f *FunctionInterpretation) traceFunctionAllocated(file string, line int) {
	tags := f.memoryTrace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", f), " ", file, ":", strconv.Itoa(line), " func_interp ",
				strconv.FormatUint(f.bytes, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (f *FunctionInterpretation) traceEvaluatorFunctionDeallocated(line int) {
	if !f.memoryTraceLive {
		return
	}
	f.memoryTraceLive = false
	tags := f.memoryTrace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !modelMemoryFinalizing(f.manager, tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", f)+" src/model/model_core.cpp:"+strconv.Itoa(line)+"\n")
			return nil
		})
}
