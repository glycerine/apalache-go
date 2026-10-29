package aig

// Source: src/tactic/aig/aig_tactic.cpp:34,39;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *Manager) memoryFinalizing(tags *diagnostic.TraceTags) bool {
	if tags.MemoryFinalizing() {
		return true
	}
	if m == nil || m.ast == nil {
		return false
	}
	numerals := m.ast.NumeralService()
	if numerals == nil {
		return false
	}
	if runtime := numerals.RuntimeOwner(); runtime != nil {
		if process := runtime.ProcessOwner(); process != nil {
			return process.Finalizing()
		}
	}
	return false
}

func (m *Manager) traceTacticAllocated() {
	m.tacticTraceLive = true
	tags := m.ast.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", m), " src/tactic/aig/aig_tactic.cpp:34 aig_manager ",
				strconv.FormatUint(m.header, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (m *Manager) traceTacticDeallocated() {
	if !m.tacticTraceLive {
		return
	}
	m.tacticTraceLive = false
	tags := m.ast.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !m.memoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", m)+" src/tactic/aig/aig_tactic.cpp:39\n")
			return nil
		})
}
