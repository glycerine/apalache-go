package ast

// Source: src/ast/ast.cpp:1364,1372-1375,1412;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"
	"unsafe"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

var astBuiltinPluginAllocation = [5]struct {
	line int
	name string
}{
	{1364, "basic_decl_plugin"},
	{1372, "label_decl_plugin"},
	{1373, "pattern_decl_plugin"},
	{1374, "model_value_decl_plugin"},
	{1375, "user_sort_plugin"},
}

func (m *ASTManager) builtinPluginTraceIdentity(id int) string {
	// The registry slice can move; the manager and source family slot cannot.
	return fmt.Sprintf("%p/%d", m, id)
}

func (m *ASTManager) traceBuiltinPluginAllocated(id int) {
	m.builtinPluginTraceLive[id] = true
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	site := astBuiltinPluginAllocation[id]
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", m.builtinPluginTraceIdentity(id), " src/ast/ast.cpp:", strconv.Itoa(site.line),
				" ", site.name, " ", strconv.FormatUint(uint64(unsafe.Sizeof(astPluginBinding{})), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (m *ASTManager) traceBuiltinPluginsDeallocated() {
	for id := range m.builtinPluginTraceLive {
		if !m.builtinPluginTraceLive[id] {
			continue
		}
		m.builtinPluginTraceLive[id] = false
		// An incomplete outer constructor has no ast_manager destructor body.
		if !m.constructorComplete {
			continue
		}
		tags := m.astMemoryTraceTags()
		if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
			continue
		}
		_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
			func() bool { return !m.astMemoryFinalizing(tags) }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "dealloc "+m.builtinPluginTraceIdentity(id)+" src/ast/ast.cpp:1412\n")
				return nil
			})
	}
}
