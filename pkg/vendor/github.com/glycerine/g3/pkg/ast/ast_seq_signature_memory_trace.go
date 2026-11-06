package ast

// Source: src/ast/seq_decl_plugin.cpp:350-352,530-572;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *astSequencePluginOwner) traceSignatureAllocated(s *astSequenceSignature, line int) {
	tags := p.owner.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", s)+
				" src/ast/seq_decl_plugin.cpp:"+strconv.Itoa(line)+" psig "+
				strconv.FormatUint(uint64(reflect.TypeOf(astSequenceSignature{}).Size()), 10)+"\n")
			return nil
		})
}

func (p *astSequencePluginOwner) traceSignatureDeallocated(s *astSequenceSignature) {
	if !s.memoryTraceLive {
		return
	}
	s.memoryTraceLive = false
	tags := p.owner.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !p.owner.astMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", s)+" src/ast/seq_decl_plugin.cpp:352\n")
			return nil
		})
}
