package numeral

// Source: src/util/rational.cpp:64-83; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindMemoryTrace borrows the generated process's memory TRACE stream. Nil
// restores the process trace state for a standalone rational global owner.
func (g *RationalGlobalService) BindMemoryTrace(tags *diagnostic.TraceTags) error {
	if e := g.open(); e != nil {
		return e
	}
	g.memoryTrace = tags
	return nil
}

func (g *RationalGlobalService) memoryTraceTags() *diagnostic.TraceTags {
	if g.memoryTrace != nil {
		return g.memoryTrace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func rationalGlobalMemoryParts(w io.Writer, parts ...string) error {
	for _, part := range parts {
		// Source ostream retains badbit and continues attempted insertions.
		_, _ = io.WriteString(w, part)
	}
	return nil
}

func (g *RationalGlobalService) traceManagerAllocated(m *RationalManager) {
	if !modularSourceDebug {
		return
	}
	tags := g.memoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return rationalGlobalMemoryParts(w, "alloc ", fmt.Sprintf("%p", m), " src/util/rational.cpp:66 synch_mpq_manager ", strconv.FormatUint(m.HeaderBytes, 10), "\n")
		})
}

func (g *RationalGlobalService) traceManagerDeallocated(m *RationalManager) {
	if !modularSourceDebug {
		return
	}
	tags := g.memoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	p := g.config.Integers.Memory.ProcessOwner()
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return p == nil || !p.Finalizing() }, func(w io.Writer) error {
			return rationalGlobalMemoryParts(w, "dealloc ", fmt.Sprintf("%p", m), " src/util/rational.cpp:82\n")
		})
}
