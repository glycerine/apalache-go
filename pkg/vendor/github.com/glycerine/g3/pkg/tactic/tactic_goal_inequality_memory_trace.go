package tactic

// Source: src/tactic/arith/propagate_ineqs_tactic.cpp:483;
// src/tactic/goal.h:85; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindInequalitySourceAllocation marks the actual goal(goal const&, bool)
// created by propagate_ineqs_tactic. The Goal owns its eventual deallocation.
func (g *Goal) BindInequalitySourceAllocation() error {
	if e := g.open(); e != nil {
		return e
	}
	if g.inequalitySourceAllocation {
		return arena.ErrInvalidSlot
	}
	g.inequalitySourceAllocation = true
	tags := g.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return nil
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", g)+
				" src/tactic/arith/propagate_ineqs_tactic.cpp:483 goal "+strconv.FormatUint(g.ownerBytes, 10)+"\n")
			return nil
		})
	return nil
}

func (g *Goal) traceSourceMemoryDeallocated() {
	if !g.inequalitySourceAllocation {
		return
	}
	g.inequalitySourceAllocation = false
	tags := g.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !g.manager.MemoryTraceFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", g)+" src/tactic/goal.h:85\n")
			return nil
		})
}
