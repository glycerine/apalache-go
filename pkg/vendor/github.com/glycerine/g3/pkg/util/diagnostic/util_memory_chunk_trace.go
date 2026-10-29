package diagnostic

// Source: src/util/memory_manager.cpp:202-211; src/util/small_object_allocator.cpp:44,60,132,221.

import (
	"io"
	"strconv"

	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// memoryChunkTrace binds the retained alloc(chunk) source site to the process
// TRACE stream. Go's stable logical chunk address replaces a physical pointer.
type memoryChunkTrace struct {
	tags    *TraceTags
	process *umemory.ProcessAccounting
}

func NewMemoryChunkTrace(tags *TraceTags, runtime *umemory.Runtime) umemory.SmallAllocatorChunkTrace {
	if !traceCompiled {
		return nil
	}
	if tags == nil {
		tags = BorrowProcessTraceTags()
	}
	var process *umemory.ProcessAccounting
	if runtime != nil {
		process = runtime.ProcessOwner()
	}
	return &memoryChunkTrace{tags: tags, process: process}
}

func memoryChunkTracePart(w io.Writer, parts ...string) error {
	for _, part := range parts {
		// ostream insertion continues after badbit; TraceTags retains the first
		// transport error and still reaches TRACE's footer and flush.
		_ = writeTracePart(w, part)
	}
	return nil
}

func (t *memoryChunkTrace) AllocatedChunk(address uint64) {
	if t == nil || !t.tags.ShouldTrace("memory") {
		return
	}
	_ = t.tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return memoryChunkTracePart(w, "alloc ", "0x"+strconv.FormatUint(address, 16),
				" ", "src/util/small_object_allocator.cpp", ":", "132", " ", "chunk", " ", "8192", "\n")
		})
}

func (t *memoryChunkTrace) DeallocatedChunk(address uint64, sourceLine int) {
	if t == nil || !t.tags.ShouldTrace("memory") {
		return
	}
	_ = t.tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return t.process == nil || !t.process.Finalizing() }, func(w io.Writer) error {
			return memoryChunkTracePart(w, "dealloc ", "0x"+strconv.FormatUint(address, 16),
				" ", "src/util/small_object_allocator.cpp", ":", strconv.Itoa(sourceLine), "\n")
		})
}
