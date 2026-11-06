package model

// Source: src/model/model.cpp:38,139; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"
	"unsafe"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (u *modelUniverse) traceAllocated() {
	tags := u.memoryTrace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", u), " src/model/model.cpp:139 ptr_vector<expr> ",
				strconv.FormatUint(uint64(unsafe.Sizeof(*u)), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (u *modelUniverse) traceDeallocated() {
	if !u.memoryTraceLive {
		return
	}
	u.memoryTraceLive = false
	tags := u.memoryTrace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !modelMemoryFinalizing(u.owner, tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", u)+" src/model/model.cpp:38\n")
			return nil
		})
}
