package ast

// Source: src/ast/ast.cpp:1702,1708,1813,1820;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ASTManager) traceInfoAllocated(r *astInfoRecord) {
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	line, name := 1708, "func_decl_info"
	if r.isSort {
		line, name = 1702, "sort_info"
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", r), " src/ast/ast.cpp:", strconv.Itoa(line),
				" ", name, " ", strconv.FormatUint(uint64(reflect.TypeOf(astInfoRecord{}).Size()), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (m *ASTManager) traceInfoDeallocated(r *astInfoRecord) {
	if !r.memoryTraceAllocated {
		return
	}
	r.memoryTraceAllocated = false
	// delete_node deliberately retains metadata when debug reference counts
	// are enabled; the later Go owning close refunds that storage only.
	if r.retainedByDebug {
		return
	}
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	line := 1820
	if r.isSort {
		line = 1813
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !m.astMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", r)+" src/ast/ast.cpp:"+strconv.Itoa(line)+"\n")
			return nil
		})
}
