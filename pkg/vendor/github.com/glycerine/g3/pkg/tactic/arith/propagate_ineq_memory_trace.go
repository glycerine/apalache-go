package arith

// Source: src/tactic/arith/propagate_ineqs_tactic.cpp:517,521,546,548;
// src/util/memory_manager.cpp:202-211.

import (
	"io"
	"strconv"
	"sync/atomic"
	"unsafe"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var inequalityMemorySerial uint64

func nextInequalityMemoryIdentity() uint64 { return atomic.AddUint64(&inequalityMemorySerial, 1) }

func (t *PropagateIneqTactic) traceImpAllocated(imp *inequalityImplementation, line int) {
	tags := t.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc 0x"+strconv.FormatUint(imp.traceID, 16)+
				" src/tactic/arith/propagate_ineqs_tactic.cpp:"+strconv.Itoa(line)+
				" imp "+strconv.FormatUint(uint64(unsafe.Sizeof(inequalityImplementation{})), 10)+"\n")
			return nil
		})
}

func (t *PropagateIneqTactic) traceImpDeallocated(imp *inequalityImplementation) {
	tags := t.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !t.manager.MemoryTraceFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc 0x"+strconv.FormatUint(imp.traceID, 16)+
				" src/tactic/arith/propagate_ineqs_tactic.cpp:"+strconv.Itoa(imp.deallocLine)+"\n")
			return nil
		})
}
