package params

// Source: src/util/params.cpp:337-339,598-607; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// BindMemoryTrace borrows the memory TRACE stream for this concrete params
// owner. Nil restores the process trace state.
func (c *ParamsContext) BindMemoryTrace(tags *udiag.TraceTags) error {
	if e := c.open(); e != nil {
		return e
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memoryTrace = tags
	return nil
}

// BindMemoryProcess borrows the generated process finalization flag.
func (c *ParamsContext) BindMemoryProcess(p *umemory.ProcessAccounting) error {
	if e := c.open(); e != nil {
		return e
	}
	if p != nil && !p.Valid() {
		return umemory.ErrRuntimeOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memoryProcess = p
	return nil
}

func (c *ParamsContext) blockMemoryTraceTags() *udiag.TraceTags {
	if c.memoryTrace != nil {
		return c.memoryTrace
	}
	return udiag.BorrowProcessTraceTags()
}

// Go block directories may move as slices grow; this source pointer is
// represented by the stable concrete owner, slot and generation identity.
func (c *ParamsContext) blockMemoryIdentity(slot uint32) string {
	return fmt.Sprintf("block[%d:%d:%d]", c.Owner, slot, c.Blocks[slot-1].Generation)
}

func (c *ParamsContext) traceBlockAllocated(slot uint32, sourceLine int) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := c.blockMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "alloc ", c.blockMemoryIdentity(slot), " src/util/params.cpp:", strconv.Itoa(sourceLine), " params ", strconv.FormatUint(paramsBlockBytes(1), 10), "\n")
		})
}

func (c *ParamsContext) traceBlockDeallocated(slot uint32) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := c.blockMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return c.memoryProcess == nil || !c.memoryProcess.Finalizing() }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "dealloc ", c.blockMemoryIdentity(slot), " src/util/params.cpp:339\n")
		})
}
