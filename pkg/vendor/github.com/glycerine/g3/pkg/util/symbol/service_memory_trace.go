package symbol

// Source: src/util/symbol.cpp:65-73; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

// BindTrace borrows the source memory TRACE stream for this symbol service.
// Nil restores the process trace state.
func (s *Service) BindTrace(t *diagnostic.TraceTags) error {
	if e := s.open(); e != nil {
		return e
	}
	s.trace = t
	return nil
}

// BindMemoryProcess borrows the generated process finalization flag.
func (s *Service) BindMemoryProcess(p *memory.ProcessAccounting) error {
	if e := s.open(); e != nil {
		return e
	}
	if p != nil && !p.Valid() {
		return memory.ErrRuntimeOwner
	}
	s.process = p
	return nil
}

func (s *Service) traceTags() *diagnostic.TraceTags {
	if s.trace != nil {
		return s.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func symbolMemoryParts(w io.Writer, parts ...string) error {
	for _, part := range parts {
		// The source ostream keeps inserting after a failed write.
		_, _ = io.WriteString(w, part)
	}
	return nil
}

func (s *Service) traceAllocated(m *Manager) {
	if !sourceDebug {
		return
	}
	tags := s.traceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return symbolMemoryParts(w, "alloc ", fmt.Sprintf("%p", m), " src/util/symbol.cpp:67 internal_symbol_table ", strconv.FormatUint(uint64(reflect.TypeOf(Manager{}).Size()), 10), "\n")
		})
}

func (s *Service) traceDeallocated(m *Manager) {
	if !sourceDebug {
		return
	}
	tags := s.traceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return s.process == nil || !s.process.Finalizing() }, func(w io.Writer) error {
			return symbolMemoryParts(w, "dealloc ", fmt.Sprintf("%p", m), " src/util/symbol.cpp:72\n")
		})
}
