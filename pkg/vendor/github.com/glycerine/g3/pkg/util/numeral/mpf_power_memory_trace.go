package numeral

// Source: src/util/mpf.h:258,268,282; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (w *arbitraryFloatPowerWrapper) tracePowerWrapperAllocated(minusOne bool) {
	tags := w.manager.floatTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	line := 268
	if minusOne {
		line = 282
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(out io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", w), " src/util/mpf.h:", strconv.Itoa(line),
				" mpz ", strconv.FormatUint(w.header, 10), "\n",
			} {
				_, _ = io.WriteString(out, part)
			}
			return nil
		})
}

func (w *arbitraryFloatPowerWrapper) tracePowerWrapperDeallocated() {
	if w.memoryTraceReleased {
		return
	}
	w.memoryTraceReleased = true
	tags := w.manager.floatTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			if tags.MemoryFinalizing() {
				return false
			}
			if runtime := w.manager.RuntimeOwner(); runtime != nil {
				if process := runtime.ProcessOwner(); process != nil && process.Finalizing() {
					return false
				}
			}
			return true
		}, func(out io.Writer) error {
			for _, part := range [...]string{
				"dealloc ", fmt.Sprintf("%p", w), " src/util/mpf.h:258\n",
			} {
				_, _ = io.WriteString(out, part)
			}
			return nil
		})
}
