package euclid

// Source: src/math/euclid/euclidean_solver.cpp:784,788,799-800;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *solverSetup) impMemoryTags() *diagnostic.TraceTags {
	if s.trace != nil {
		return s.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (s *solverSetup) traceImpAllocated(line int) {
	tags := s.impMemoryTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", s)+
				" src/math/euclid/euclidean_solver.cpp:"+strconv.Itoa(line)+" imp "+
				strconv.FormatUint(s.header, 10)+"\n")
			return nil
		})
}

func (s *solverSetup) traceImpDeallocated() {
	tags := s.impMemoryTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !tags.MemoryFinalizing() }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", s)+
				" src/math/euclid/euclidean_solver.cpp:"+strconv.Itoa(s.memoryDeallocLine)+"\n")
			return nil
		})
}
