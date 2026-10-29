package proofs

// Source: src/ast/proofs/proof_utils.cpp:365,398,412,517,569;
// src/util/util.h:223-225; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Each allocated C++ expr_set has a distinct owner even when multiple proof
// entries borrow the same set. Go maps hold the keys; this record carries the
// concrete allocation identity and lifetime of the retained set owner.
type hypothesisSetOwner struct {
	values   map[ast.ASTHandle]struct{}
	bytes    uint64
	released bool
}

func (h *hypothesisSets) newOwnedSet(line int, inherited map[ast.ASTHandle]struct{}) (*hypothesisSetOwner, error) {
	size := uint64(reflect.TypeOf(hypothesisSetOwner{}).Size())
	if err := h.manager.BudgetField.Reserve(size); err != nil {
		return nil, err
	}
	owner := &hypothesisSetOwner{values: make(map[ast.ASTHandle]struct{}, len(inherited)), bytes: size}
	for key := range inherited {
		owner.values[key] = struct{}{}
	}
	h.owned = append(h.owned, owner)
	h.traceOwnedSetAllocated(owner, line)
	return owner, nil
}

func (h *hypothesisSets) releaseOwnedSet(owner *hypothesisSetOwner, source string, line int) {
	if owner == nil || owner.released {
		return
	}
	h.manager.BudgetField.Release(owner.bytes)
	owner.bytes = 0
	owner.released = true
	h.traceOwnedSetDeallocated(owner, source, line)
	owner.values = nil
}

func (h *hypothesisSets) traceOwnedSetAllocated(owner *hypothesisSetOwner, line int) {
	tags := h.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", owner)+
				" src/ast/proofs/proof_utils.cpp:"+strconv.Itoa(line)+
				" expr_set "+strconv.FormatUint(uint64(reflect.TypeOf(hypothesisSetOwner{}).Size()), 10)+"\n")
			return nil
		})
}

func (h *hypothesisSets) traceOwnedSetDeallocated(owner *hypothesisSetOwner, source string, line int) {
	tags := h.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !h.manager.MemoryTraceFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", owner)+" "+source+":"+strconv.Itoa(line)+"\n")
			return nil
		})
}
