package pattern

// Source: src/ast/pattern/pattern_inference.cpp:186,188,205,238,263-267;
// src/util/util.h:171; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *patternCandidates) traceCollectedInfoAllocated(info *collectedInfo, line int) {
	tags := c.patternMemoryTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", info)+" src/ast/pattern/pattern_inference.cpp:"+
				strconv.Itoa(line)+" info "+strconv.FormatUint(uint64(reflect.TypeOf(collectedInfo{}).Size()), 10)+"\n")
			return nil
		})
}

func (c *patternCandidates) traceCollectedInfoDeallocated(info *collectedInfo) {
	tags := c.patternMemoryTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !c.patternMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", info)+" src/util/util.h:171\n")
			return nil
		})
}
