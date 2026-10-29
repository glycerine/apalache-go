package stopwatch

// Source: src/util/timer.cpp:24-31; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"
	"unsafe"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *Timer) traceWatchAllocated() {
	if !diagnostic.DebugBuild || !p.memoryTrace.ShouldTrace("memory") {
		return
	}
	_ = p.memoryTrace.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", &p.watch), " src/util/timer.cpp:25 stopwatch ",
				strconv.FormatUint(uint64(unsafe.Sizeof(p.watch)), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (p *Timer) traceWatchDeallocated() {
	if !diagnostic.DebugBuild || !p.memoryTrace.ShouldTrace("memory") {
		return
	}
	_ = p.memoryTrace.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !p.memoryTrace.MemoryFinalizing() }, func(w io.Writer) error {
			for _, part := range [...]string{
				"dealloc ", fmt.Sprintf("%p", &p.watch), " src/util/timer.cpp:30\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}
