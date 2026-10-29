package ast

// Source: src/ast/reg_decl_plugins.cpp:32; src/ast/ast.cpp:1412;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *astArithmeticPluginOwner) traceArithmeticPluginAllocated() {
	p.memoryTraceLive = true
	tags := p.owner.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", fmt.Sprintf("%p", p), " src/ast/reg_decl_plugins.cpp:32 arith_decl_plugin ",
				strconv.FormatUint(p.header, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (p *astArithmeticPluginOwner) traceArithmeticPluginDeallocated() {
	if !p.memoryTraceLive {
		return
	}
	p.memoryTraceLive = false
	// Only a published plugin is reached by ast_manager::~ast_manager.
	if !p.published || !p.owner.constructorComplete {
		return
	}
	tags := p.owner.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !p.owner.astMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", p)+" src/ast/ast.cpp:1412\n")
			return nil
		})
}
