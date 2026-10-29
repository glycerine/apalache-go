package params

// Source: src/util/params.cpp:753-761,992-1014; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The source rational* is an inline Go value that may move as entry pages
// grow or reset-key shifts suffixes. Its owning setter invocation is stable.
func (c *ParamsContext) rationalEntryMemoryIdentity(id uint64) string {
	return fmt.Sprintf("rational[%d:%d]", c.Owner, id)
}

func (c *ParamsContext) traceRationalEntryAllocated(id uint64, sourceLine int) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := c.blockMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "alloc ", c.rationalEntryMemoryIdentity(id), " src/util/params.cpp:", strconv.Itoa(sourceLine), " rational ", strconv.FormatUint(uint64(reflect.TypeOf(unumeral.OwnedRational{}).Size()), 10), "\n")
		})
}

func (c *ParamsContext) traceRationalEntryDeallocated(id uint64) {
	if !GlobalParamsSourceDebug || id == 0 {
		return
	}
	tags := c.blockMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return c.memoryProcess == nil || !c.memoryProcess.Finalizing() }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "dealloc ", c.rationalEntryMemoryIdentity(id), " src/util/params.cpp:757\n")
		})
}
