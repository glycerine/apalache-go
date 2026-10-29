package polynomial

// Source: src/math/polynomial/polynomial.cpp:783,803;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MonomialManager) traceOwnedAllocatorAllocated() {
	tags := m.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", m.allocator)+
				" src/math/polynomial/polynomial.cpp:783 small_object_allocator "+
				strconv.FormatUint(m.allocator.Stats().OwnerBytes, 10)+"\n")
			return nil
		})
}

func (m *MonomialManager) traceOwnedAllocatorDeallocated() {
	tags := m.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			if tags.MemoryFinalizing() {
				return false
			}
			process := m.memory.ProcessOwner()
			return process == nil || !process.Finalizing()
		}, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", m.allocator)+" src/math/polynomial/polynomial.cpp:803\n")
			return nil
		})
}
