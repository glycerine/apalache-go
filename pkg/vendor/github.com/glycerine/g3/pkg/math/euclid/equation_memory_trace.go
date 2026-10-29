package euclid

// Source: src/math/euclid/euclidean_solver.cpp:155,303;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (e *equation) memoryTraceTags() *diagnostic.TraceTags {
	if e.trace != nil {
		return e.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (e *equation) traceAllocated() {
	tags := e.memoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", e)+
				" src/math/euclid/euclidean_solver.cpp:155 equation "+
				strconv.FormatUint(e.header, 10)+"\n")
			return nil
		})
}

func (e *equation) traceDeallocated() {
	tags := e.memoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !tags.MemoryFinalizing() }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", e)+
				" src/math/euclid/euclidean_solver.cpp:303\n")
			return nil
		})
}
