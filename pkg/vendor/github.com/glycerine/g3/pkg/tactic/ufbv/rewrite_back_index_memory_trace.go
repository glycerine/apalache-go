package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:485;
// src/util/obj_hashtable.h:216-222; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (r *rewriteBackIndex) ensureBackSet(f ast.ASTHandle) (*rewriteBackSet, error) {
	if set, found := r.byDecl[f]; found {
		if set == nil {
			if e := assertBackIndexSetPresent(482); e != nil {
				return nil, e
			}
			return nil, ast.ErrASTDomain
		}
		return set, nil
	}
	n := uint64(reflect.TypeOf(rewriteBackSet{}).Size())
	if e := r.manager.Budget().Reserve(n); e != nil {
		return nil, e
	}
	set := &rewriteBackSet{values: make(map[ast.ASTHandle]struct{}), header: n}
	r.byDecl[f] = set
	tags := r.manager.BorrowMemoryTraceTags()
	if diagnostic.DebugBuild && tags.ShouldTrace("memory") {
		_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
			func() bool { return true }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", set)+" src/tactic/ufbv/ufbv_rewriter.cpp:485 expr_set "+strconv.FormatUint(n, 10)+"\n")
				return nil
			})
	}
	return set, nil
}

func (r *rewriteBackIndex) releaseBackSet(set *rewriteBackSet) {
	if set == nil || set.header == 0 {
		return
	}
	tags := r.manager.BorrowMemoryTraceTags()
	if diagnostic.DebugBuild && tags.ShouldTrace("memory") {
		_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
			func() bool {
				if tags.MemoryFinalizing() {
					return false
				}
				numerals := r.manager.NumeralService()
				if numerals == nil || numerals.RuntimeOwner() == nil {
					return true
				}
				process := numerals.RuntimeOwner().ProcessOwner()
				return process == nil || !process.Finalizing()
			}, func(w io.Writer) error {
				_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", set)+" src/util/obj_hashtable.h:220\n")
				return nil
			})
	}
	r.manager.Budget().Release(set.header)
	set.header = 0
	set.values = nil
}
