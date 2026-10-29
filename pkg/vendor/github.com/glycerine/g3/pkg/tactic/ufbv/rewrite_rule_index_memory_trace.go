package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:226;
// src/util/obj_hashtable.h:216-222; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (r *rewriteRuleIndex) newForwardSet() (*rewriteForwardSet, error) {
	n := uint64(reflect.TypeOf(rewriteForwardSet{}).Size())
	if e := r.manager.Budget().Reserve(n); e != nil {
		return nil, e
	}
	set := &rewriteForwardSet{header: n}
	tags := r.manager.BorrowMemoryTraceTags()
	if diagnostic.DebugBuild && tags.ShouldTrace("memory") {
		_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
			func() bool { return true }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", set)+" src/tactic/ufbv/ufbv_rewriter.cpp:226 quantifier_set "+strconv.FormatUint(n, 10)+"\n")
				return nil
			})
	}
	return set, nil
}

func (r *rewriteRuleIndex) forwardSetMemoryFinalizing(tags *diagnostic.TraceTags) bool {
	if tags.MemoryFinalizing() {
		return true
	}
	numerals := r.manager.NumeralService()
	if numerals == nil || numerals.RuntimeOwner() == nil {
		return false
	}
	process := numerals.RuntimeOwner().ProcessOwner()
	return process != nil && process.Finalizing()
}

func (r *rewriteRuleIndex) releaseForwardSet(set *rewriteForwardSet) {
	if set == nil || set.header == 0 {
		return
	}
	tags := r.manager.BorrowMemoryTraceTags()
	if diagnostic.DebugBuild && tags.ShouldTrace("memory") {
		_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
			func() bool { return !r.forwardSetMemoryFinalizing(tags) }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", set)+" src/util/obj_hashtable.h:220\n")
				return nil
			})
	}
	r.manager.Budget().Release(set.header)
	set.header = 0
	set.values = nil
}
