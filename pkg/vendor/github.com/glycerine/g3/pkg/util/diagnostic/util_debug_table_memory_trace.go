package diagnostic

// Source: src/util/debug.cpp:45-54; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
)

// BindMemoryTrace borrows the trace stream for this debug owner and returns
// the prior binding so a generated process can restore its caller's state.
func (d *DebugContext) BindMemoryTrace(tags *TraceTags) (*TraceTags, error) {
	if err := d.open(); err != nil {
		return nil, err
	}
	previous := d.memoryTrace
	d.memoryTrace = tags
	return previous, nil
}

func (d *DebugContext) debugMemoryTags() *TraceTags {
	if d.memoryTrace != nil {
		return d.memoryTrace
	}
	return BorrowProcessTraceTags()
}

func (d *DebugContext) traceDebugTableAllocated() {
	if !sourceDebug {
		return
	}
	tags := d.debugMemoryTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", &d.Tags), " src/util/debug.cpp:48 str_hashtable ",
				strconv.FormatUint(uint64(reflect.TypeOf(map[string]struct{}{}).Size()), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (d *DebugContext) traceDebugTableDeallocated() {
	if !sourceDebug {
		return
	}
	tags := d.debugMemoryTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return tags.memoryProcess == nil || !tags.memoryProcess.Finalizing() }, func(w io.Writer) error {
			for _, part := range [...]string{
				"dealloc ", fmt.Sprintf("%p", &d.Tags), " src/util/debug.cpp:53\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}
