package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter_tactic.cpp:81,89,113-115;
// src/util/memory_manager.cpp:202-211.

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *RewriterTactic) newImp(line int) (*rewriteTacticImp, error) {
	n := uint64(reflect.TypeOf(rewriteTacticImp{}).Size())
	if e := t.manager.Budget().Reserve(n); e != nil {
		return nil, e
	}
	imp := &rewriteTacticImp{manager: t.manager, header: n}
	tags := t.manager.BorrowMemoryTraceTags()
	if diagnostic.DebugBuild && tags.ShouldTrace("memory") {
		_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
			func() bool { return true }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", imp)+" src/tactic/ufbv/ufbv_rewriter_tactic.cpp:"+strconv.Itoa(line)+" imp "+strconv.FormatUint(n, 10)+"\n")
				return nil
			})
	}
	local, e := t.context.NewValue()
	if e != nil {
		return imp, e
	}
	key := "elim_and"
	if e = t.context.SetBoolName(&local, &key, true); e != nil {
		return imp, errors.Join(e, t.context.Del(&local))
	}
	imp.boolean, e = rewriter.NewBoolRewriter(t.manager, t.context, t.globals, &local)
	return imp, errors.Join(e, t.context.Del(&local))
}

func (t *RewriterTactic) impMemoryFinalizing(tags *diagnostic.TraceTags) bool {
	if tags.MemoryFinalizing() {
		return true
	}
	numerals := t.manager.NumeralService()
	if numerals == nil || numerals.RuntimeOwner() == nil {
		return false
	}
	process := numerals.RuntimeOwner().ProcessOwner()
	return process != nil && process.Finalizing()
}

func (t *RewriterTactic) releaseImp(imp *rewriteTacticImp, line int) error {
	if imp == nil || imp.header == 0 {
		return nil
	}
	if imp.boolean != nil {
		if e := imp.boolean.Close(); e != nil {
			return e
		}
		imp.boolean = nil
	}
	tags := t.manager.BorrowMemoryTraceTags()
	if diagnostic.DebugBuild && tags.ShouldTrace("memory") {
		_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
			func() bool { return !t.impMemoryFinalizing(tags) }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", imp)+" src/tactic/ufbv/ufbv_rewriter_tactic.cpp:"+strconv.Itoa(line)+"\n")
				return nil
			})
	}
	t.manager.Budget().Release(imp.header)
	imp.header = 0
	return nil
}
