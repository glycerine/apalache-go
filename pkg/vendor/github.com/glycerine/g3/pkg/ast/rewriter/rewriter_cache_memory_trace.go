package rewriter

// Source: src/ast/rewriter/rewriter.cpp:25,29,99,101;
// src/util/util.h:169-173; src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *rewriterCore) traceCacheAllocated(cache *ast.ActivityCache, line int) {
	tags := c.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", cache)+
				" src/ast/rewriter/rewriter.cpp:"+strconv.Itoa(line)+
				" cache "+strconv.FormatUint(uint64(reflect.TypeOf(cache).Elem().Size()), 10)+"\n")
			return nil
		})
}

func (c *rewriterCore) traceCacheDeallocated(cache *ast.ActivityCache) {
	tags := c.manager.BorrowMemoryTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !c.manager.MemoryTraceFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", cache)+" src/util/util.h:171\n")
			return nil
		})
}
