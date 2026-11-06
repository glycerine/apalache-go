package api

// Source: src/api/api_goal.cpp:35; src/api/api_context.cpp:54,116;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *apiContextResults) traceGoalAllocated(g *apiGoalObject, bytes uint64) {
	if !uparams.GlobalParamsSourceDebug || !c.trace.ShouldTrace("memory") {
		return
	}
	_ = c.trace.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range []string{"alloc ", fmt.Sprintf("%p", g), " src/api/api_goal.cpp:35 Z3_goal_ref ", strconv.FormatUint(bytes, 10), "\n"} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (c *apiContextResults) traceGoalDeallocated(g *apiGoalObject, line int) {
	if !uparams.GlobalParamsSourceDebug || !c.trace.ShouldTrace("memory") {
		return
	}
	_ = c.trace.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return c.memoryProcess == nil || !c.memoryProcess.Finalizing() }, func(w io.Writer) error {
			for _, part := range []string{"dealloc ", fmt.Sprintf("%p", g), " src/api/api_context.cpp:", strconv.Itoa(line), "\n"} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}
