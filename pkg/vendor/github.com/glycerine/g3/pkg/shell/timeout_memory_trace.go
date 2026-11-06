package shell

// Source: src/util/scoped_timer.cpp:255-265; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"
	"unsafe"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type timeoutMemoryTrace struct {
	tags *diagnostic.TraceTags
}

func newTimeoutMemoryTrace(tags *diagnostic.TraceTags) control.ScopedTimerMemoryTrace {
	if !shellDebugOptions {
		return nil
	}
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	return &timeoutMemoryTrace{tags: tags}
}

func (s *TimeoutService) BindMemoryTrace(tags *diagnostic.TraceTags) error {
	if s == nil || s.self != s {
		return control.ErrOwner
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return control.ErrOwner
	}
	// A live timer retains the trace owner chosen at its allocation. The
	// generated process binds its finalization flag to that owner directly.
	s.trace = tags
	return nil
}

func timeoutMemoryParts(w io.Writer, parts ...string) error {
	for _, part := range parts {
		// ostream insertions continue after badbit. TraceTags records the
		// transport error and still reaches the footer and flush.
		_, _ = io.WriteString(w, part)
	}
	return nil
}

func (t *timeoutMemoryTrace) AllocatedScopedTimer(timer *control.ScopedTimer) {
	if t == nil || !t.tags.ShouldTrace("memory") {
		return
	}
	_ = t.tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return timeoutMemoryParts(w, "alloc ", fmt.Sprintf("%p", timer), " src/util/scoped_timer.cpp:257 imp ", strconv.FormatUint(uint64(unsafe.Sizeof(*timer)), 10), "\n")
		})
}

func (t *timeoutMemoryTrace) DeallocatedScopedTimer(timer *control.ScopedTimer) {
	if t == nil || !t.tags.ShouldTrace("memory") {
		return
	}
	_ = t.tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !t.tags.MemoryFinalizing() }, func(w io.Writer) error {
			return timeoutMemoryParts(w, "dealloc ", fmt.Sprintf("%p", timer), " src/util/scoped_timer.cpp:264\n")
		})
}
