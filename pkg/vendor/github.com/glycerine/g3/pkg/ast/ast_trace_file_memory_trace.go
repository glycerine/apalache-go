package ast

// Source: src/ast/ast.cpp:1308,1478; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (f *astTraceFile) memoryTraceTags() *udiag.TraceTags {
	if f.trace != nil {
		return f.trace
	}
	return udiag.BorrowProcessTraceTags()
}

func (f *astTraceFile) traceAllocated() {
	tags := f.memoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", f), " src/ast/ast.cpp:1308 std::fstream ",
				strconv.FormatUint(f.header, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (f *astTraceFile) traceDeallocated() {
	tags := f.memoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !f.owner.astMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", f)+" src/ast/ast.cpp:1478\n")
			return nil
		})
}
