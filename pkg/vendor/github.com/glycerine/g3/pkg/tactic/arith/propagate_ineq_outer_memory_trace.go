package arith

// Source: src/tactic/arith/propagate_ineqs_tactic.cpp:47,61;
// src/tactic/tactic.h:43; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"
	"unsafe"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *PropagateIneqTactic) traceOuterAllocated() {
	tags := t.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", t)+
				" src/tactic/arith/propagate_ineqs_tactic.cpp:"+strconv.Itoa(t.outerSourceLine)+
				" propagate_ineqs_tactic "+strconv.FormatUint(uint64(unsafe.Sizeof(PropagateIneqTactic{})), 10)+"\n")
			return nil
		})
}

func (t *PropagateIneqTactic) traceOuterDeallocated() {
	tags := t.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !t.manager.MemoryTraceFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", t)+" src/tactic/tactic.h:43\n")
			return nil
		})
}
