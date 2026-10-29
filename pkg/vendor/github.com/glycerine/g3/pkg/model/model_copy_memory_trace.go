package model

// Source: src/model/model.cpp:69; src/tactic/model_converter.cpp:178;
// src/tactic/filter_model_converter.cpp:31;
// src/sat/tactic/sat_tactic.cpp:93;
// src/model/model_core.h:71-75;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (md *Model) traceSourceAllocated(file string, line int) {
	md.allocationTrace = md.CoreField.manager.BorrowMemoryTraceTags()
	md.allocationLive = true
	tags := md.allocationTrace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", md), " ", file, ":", strconv.Itoa(line), " model ",
				strconv.FormatUint(md.ownerBytes, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (md *Model) traceSourceDeallocated() {
	if !md.allocationLive {
		return
	}
	md.allocationLive = false
	tags := md.allocationTrace
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !modelMemoryFinalizing(md.CoreField.manager, tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", md)+" src/model/model_core.h:74\n")
			return nil
		})
}
