package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter_tactic.cpp:121;
// src/tactic/tactic.h:43; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *RewriterTactic) traceOuterAllocated() {
	tags := t.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", t)+" src/tactic/ufbv/ufbv_rewriter_tactic.cpp:"+strconv.Itoa(t.allocationLine)+" ufbv_rewriter_tactic "+strconv.FormatUint(t.header, 10)+"\n")
			return nil
		})
}

func (t *RewriterTactic) traceOuterDeallocated() {
	if t.header == 0 {
		return
	}
	tags := t.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !t.impMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", t)+" src/tactic/tactic.h:43\n")
			return nil
		})
}
