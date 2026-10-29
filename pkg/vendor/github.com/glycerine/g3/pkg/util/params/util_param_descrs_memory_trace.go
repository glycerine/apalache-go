package params

// Source: src/util/params.cpp:209-216; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (d *ParamDescrs) impMemoryTraceTags() *udiag.TraceTags {
	if d.memoryOwner != nil {
		return d.memoryOwner.moduleMemoryTraceTags()
	}
	return udiag.BorrowProcessTraceTags()
}

func (d *ParamDescrs) traceImpAllocated() {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := d.impMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "alloc ", fmt.Sprintf("%p", &d.entries), " src/util/params.cpp:211 imp ", strconv.FormatUint(uint64(reflect.TypeOf(ParamDescrs{}).Size()), 10), "\n")
		})
}

func (d *ParamDescrs) traceImpDeallocated() {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := d.impMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			return d.memoryOwner == nil || d.memoryOwner.memoryProcess == nil || !d.memoryOwner.memoryProcess.Finalizing()
		}, func(w io.Writer) error {
			return gparamsMemoryParts(w, "dealloc ", fmt.Sprintf("%p", &d.entries), " src/util/params.cpp:215\n")
		})
}
