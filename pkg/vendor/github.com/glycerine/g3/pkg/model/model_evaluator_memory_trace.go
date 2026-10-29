package model

// Source: src/model/model_evaluator.cpp:523,531,556-557;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (ev *ModelEvaluator) impTraceTags() *diagnostic.TraceTags {
	if ev.services.Trace != nil {
		return ev.services.Trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (ev *ModelEvaluator) impTraceIdentity() string {
	// Cleanup replaces the source imp while retaining the Go evaluator owner.
	return fmt.Sprintf("%p/%d", ev, ev.impGeneration)
}

func (ev *ModelEvaluator) traceImpAllocated(line int) {
	tags := ev.impTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", ev.impTraceIdentity(), " src/model/model_evaluator.cpp:",
				strconv.Itoa(line),
				" imp ", strconv.FormatUint(ev.header, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (ev *ModelEvaluator) traceImpDeallocated(line int) {
	if !ev.impLive {
		return
	}
	tags := ev.impTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			if tags.MemoryFinalizing() {
				return false
			}
			if process := ev.services.Memory.ProcessOwner(); process != nil && process.Finalizing() {
				return false
			}
			return true
		}, func(w io.Writer) error {
			for _, part := range [...]string{
				"dealloc ", ev.impTraceIdentity(), " src/model/model_evaluator.cpp:", strconv.Itoa(line), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}
