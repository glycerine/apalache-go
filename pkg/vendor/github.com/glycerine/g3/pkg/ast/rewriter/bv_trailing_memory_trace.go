package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:342,376,384,388;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *BVTrailing) traceImpAllocated() {
	tags := p.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", p)+
				" src/ast/rewriter/bv_trailing.cpp:384 imp "+
				strconv.FormatUint(uint64(reflect.TypeOf(BVTrailing{}).Size()), 10)+"\n")
			return nil
		})
}

func (p *BVTrailing) traceImpDeallocated() {
	tags := p.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !p.manager.MemoryTraceFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", p)+" src/ast/rewriter/bv_trailing.cpp:388\n")
			return nil
		})
}

func (p *BVTrailing) traceTableAllocated(table *bvTrailingTable) {
	tags := p.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", table)+
				" src/ast/rewriter/bv_trailing.cpp:342 map "+
				strconv.FormatUint(uint64(reflect.TypeOf(bvTrailingTable{}).Size()), 10)+"\n")
			return nil
		})
}

func (p *BVTrailing) traceTableDeallocated(table *bvTrailingTable) {
	tags := p.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !p.manager.MemoryTraceFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", table)+" src/ast/rewriter/bv_trailing.cpp:376\n")
			return nil
		})
}
