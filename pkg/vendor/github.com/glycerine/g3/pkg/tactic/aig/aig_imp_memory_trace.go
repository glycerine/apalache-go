package aig

// Source: src/tactic/aig/aig.cpp:1684,1688;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *Manager) traceImpAllocated() {
	m.impTraceLive = true
	tags := m.ast.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", m.managerImp), " src/tactic/aig/aig.cpp:1684 imp ",
				strconv.FormatUint(m.impHeader, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (m *Manager) traceImpDeallocated() {
	if !m.impTraceLive {
		return
	}
	m.impTraceLive = false
	tags := m.ast.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !m.memoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", m.managerImp)+" src/tactic/aig/aig.cpp:1688\n")
			return nil
		})
}
