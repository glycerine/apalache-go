package diagnostic

// Source: src/util/trace.cpp:28-33; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
)

// BindMemoryProcess borrows the process finalization guard used by the source
// debug deallocation wrapper. Standalone tag owners may leave it nil.
func (t *TraceTags) BindMemoryProcess(p *memory.ProcessAccounting) error {
	if t == nil {
		return arena.ErrConfig
	}
	if p != nil && !p.Valid() {
		return memory.ErrRuntimeOwner
	}
	t.memoryProcess = p
	return nil
}

// MemoryFinalizing exposes the process guard already bound to this trace
// stream, so concrete allocation sites share the source global state.
func (t *TraceTags) MemoryFinalizing() bool {
	return t != nil && t.memoryProcess != nil && t.memoryProcess.Finalizing()
}

// The source allocation is traced before g_enabled_trace_tags is assigned.
// Thus only enable_all_trace(true), not a tag in the new table, can enable it.
func (t *TraceTags) traceTagTableAllocated() {
	if !sourceDebug || !t.ShouldTrace("memory") {
		return
	}
	_ = t.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", &t.tags), " src/util/trace.cpp:31 str_hashtable ",
				strconv.FormatUint(uint64(reflect.TypeOf(map[string]struct{}{}).Size()), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

// Source deallocf destroys the table before memory::deallocate checks the
// trace tag. Only the all flag can select a defined trace after destruction.
func (t *TraceTags) traceTagTableDeallocated() {
	if !sourceDebug || !t.all || !traceCompiled {
		return
	}
	_ = t.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return t.memoryProcess == nil || !t.memoryProcess.Finalizing() }, func(w io.Writer) error {
			for _, part := range [...]string{
				"dealloc ", fmt.Sprintf("%p", &t.tags), " src/util/trace.cpp:37\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}
