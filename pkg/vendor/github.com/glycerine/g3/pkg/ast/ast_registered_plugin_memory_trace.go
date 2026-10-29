package ast

// Source: src/ast/reg_decl_plugins.cpp:30-55; src/ast/ast.cpp:1412;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *astArithmeticPluginOwner) traceIdentity() string    { return fmt.Sprintf("%p", p) }
func (p *astBitVectorPluginOwner) traceIdentity() string     { return fmt.Sprintf("%p", p) }
func (p *astArrayPluginOwner) traceIdentity() string         { return fmt.Sprintf("%p", p) }
func (p *astDatatypePluginOwner) traceIdentity() string      { return fmt.Sprintf("%p", p) }
func (p *astAuxiliaryPluginOwner) traceIdentity() string     { return fmt.Sprintf("%p", p) }
func (p *astSequencePluginOwner) traceIdentity() string      { return fmt.Sprintf("%p", p) }
func (p *astFloatingPointPluginOwner) traceIdentity() string { return fmt.Sprintf("%p", p) }
func (p *astFormatPluginOwner) traceIdentity() string        { return fmt.Sprintf("%p", p) }

// These records describe the concrete charged plugin headers. Each caller
// supplies its original allocation site and retains its own lifetime flag.
func (m *ASTManager) traceRegisteredPluginAllocated(identity string, extent uint64, line int, name string, live *bool) {
	m.traceRegisteredPluginAllocatedAt(identity, extent, "src/ast/reg_decl_plugins.cpp", line, name, live)
}

func (m *ASTManager) traceRegisteredPluginAllocatedAt(identity string, extent uint64, file string, line int, name string, live *bool) {
	*live = true
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", identity, " ", file, ":", strconv.Itoa(line),
				" ", name, " ", strconv.FormatUint(extent, 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (m *ASTManager) traceRegisteredPluginDeallocated(identity string, published bool, live *bool) {
	if !*live {
		return
	}
	*live = false
	if !published || !m.constructorComplete {
		return
	}
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !m.astMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+identity+" src/ast/ast.cpp:1412\n")
			return nil
		})
}

// ast_manager deletes its registered plugins in family order after all
// finalize calls. Header storage can be released separately during Go cleanup.
func (m *ASTManager) traceRegisteredPluginsDeallocated() {
	for id := range m.plugins.bindings {
		binding := m.plugins.bindings[id]
		if !binding.present {
			continue
		}
		switch binding.kind {
		case astFormatPlugin:
			if p := m.formatPlugin; p != nil {
				m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
			}
		case astArithmeticPlugin:
			if p := m.arithmeticPlugin; p != nil {
				p.traceArithmeticPluginDeallocated()
			}
		case astBitVectorPlugin:
			if p := m.bitVectorPlugin; p != nil {
				m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
			}
		case astArrayPlugin:
			if p := m.arrayPlugin; p != nil {
				m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
			}
		case astDatatypePlugin:
			if p := m.datatypePlugin; p != nil {
				m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
			}
		case astDatalogPlugin, astPseudoBooleanPlugin:
			if p := m.auxiliaryPlugin(binding.kind); p != nil {
				m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
			}
		case astSequencePlugin:
			if p := m.sequencePlugin; p != nil {
				m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
			}
		case astFloatingPointPlugin:
			if p := m.floatingPointPlugin; p != nil {
				m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
			}
		}
	}
}
