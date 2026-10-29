package ast

// Source: src/ast/ast.cpp:1313,1330,1473; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ASTManager) astMemoryTraceTags() *udiag.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return udiag.BorrowProcessTraceTags()
}

// A standalone AST manager can borrow an unbound trace stream while its
// numeral service still belongs to a process. The source memory guard follows
// that owning process, including during its finalization interval.
func (m *ASTManager) astMemoryFinalizing(tags *udiag.TraceTags) bool {
	if tags.MemoryFinalizing() {
		return true
	}
	if m == nil || m.numerals == nil {
		return false
	}
	runtime := m.numerals.RuntimeOwner()
	return runtime != nil && runtime.ProcessOwner() != nil && runtime.ProcessOwner().Finalizing()
}

// BorrowMemoryTraceTags lets retained child owners emit source memory records
// through the same trace channel as their AST manager.
func (m *ASTManager) BorrowMemoryTraceTags() *udiag.TraceTags {
	return m.astMemoryTraceTags()
}

// MemoryTraceFinalizing lets retained child owners use the manager's process
// finalization gate for source memory deallocation records.
func (m *ASTManager) MemoryTraceFinalizing(tags *udiag.TraceTags) bool {
	return m.astMemoryFinalizing(tags)
}

func (m *ASTManager) traceFormatAllocated(line int) {
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", m), " src/ast/ast.cpp:", strconv.Itoa(line),
				" ast_manager ", strconv.FormatUint(m.ownerBytes, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (m *ASTManager) traceFormatDeallocated() {
	if !m.formatTraceLive {
		return
	}
	m.formatTraceLive = false
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !m.astMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", m)+" src/ast/ast.cpp:1473\n")
			return nil
		})
}
