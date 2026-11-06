package stopwatch

// Source: src/util/timeit.cpp:48-58; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"
	"unsafe"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *Timeit) traceImpAllocated() {
	tags := p.trace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", p), " src/util/timeit.cpp:50 imp ",
				strconv.FormatUint(uint64(unsafe.Sizeof(*p)), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (p *Timeit) traceImpDeallocated() {
	tags := p.trace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !tags.MemoryFinalizing() }, func(w io.Writer) error {
			for _, part := range [...]string{
				"dealloc ", fmt.Sprintf("%p", p), " src/util/timeit.cpp:57\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}
