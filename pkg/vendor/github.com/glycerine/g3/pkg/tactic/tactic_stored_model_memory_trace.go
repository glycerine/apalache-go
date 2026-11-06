package tactic

// Source: src/tactic/model_converter.cpp:167,173; src/tactic/converter.h:36;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *tacticStoredModelConverter) traceStoredModelAllocated(withLabels bool) {
	c.memoryTraceLive = true
	tags := c.memoryTrace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	line := 167
	if withLabels {
		line = 173
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", c)+" src/tactic/model_converter.cpp:"+
				strconv.Itoa(line)+" model2mc "+strconv.FormatUint(c.ownerBytes, 10)+"\n")
			return nil
		})
}

func (c *tacticStoredModelConverter) memoryFinalizing() bool {
	if c.memoryTrace.MemoryFinalizing() {
		return true
	}
	if c.memoryManager == nil {
		return false
	}
	numerals := c.memoryManager.NumeralService()
	if numerals == nil || numerals.RuntimeOwner() == nil {
		return false
	}
	process := numerals.RuntimeOwner().ProcessOwner()
	return process != nil && process.Finalizing()
}

func (c *tacticStoredModelConverter) traceStoredModelDeallocated() {
	if !c.memoryTraceLive {
		return
	}
	c.memoryTraceLive = false
	tags := c.memoryTrace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !c.memoryFinalizing() }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", c)+" src/tactic/converter.h:36\n")
			return nil
		})
}
