package params

// Source: src/util/memory_manager.cpp:202-211; src/util/gparams.cpp:633-644.

import (
	"fmt"
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// BindMemoryProcess borrows the source process finalization flag. Nil is used
// by standalone parameter owners that do not have a process runtime.
func (s *GlobalParamsService) BindMemoryProcess(p *umemory.ProcessAccounting) error {
	if e := s.open(); e != nil {
		return e
	}
	if p != nil && !p.Valid() {
		return umemory.ErrRuntimeOwner
	}
	if e := s.Context.BindMemoryProcess(p); e != nil {
		return e
	}
	s.memoryProcess = p
	for _, head := range []*GlobalParams{s.Current, s.Superseded, s.Failed} {
		for g := head; g != nil; g = g.RetainedNext {
			g.memoryProcess = p
		}
	}
	return nil
}

func (s *GlobalParamsService) memoryTraceTags() *udiag.TraceTags {
	if s.namespaceTrace != nil {
		return s.namespaceTrace
	}
	return udiag.BorrowProcessTraceTags()
}

func gparamsMemoryParts(w io.Writer, parts ...string) error {
	for _, part := range parts {
		// Source ostream keeps inserting after badbit; TraceTags retains the
		// first transport failure and still attempts the footer and flush.
		_, _ = io.WriteString(w, part)
	}
	return nil
}

func (s *GlobalParamsService) traceMemoryAllocated(g *GlobalParams) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := s.memoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "alloc ", fmt.Sprintf("%p", g), " src/util/gparams.cpp:635 imp ", strconv.FormatUint(g.OwnerBytes, 10), "\n")
		})
}

func (s *GlobalParamsService) traceMemoryDeallocated(g *GlobalParams) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := s.memoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return s.memoryProcess == nil || !s.memoryProcess.Finalizing() }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "dealloc ", fmt.Sprintf("%p", g), " src/util/gparams.cpp:641\n")
		})
}

func (g *GlobalParams) moduleMemoryTraceTags() *udiag.TraceTags {
	if g.memoryTrace != nil {
		return g.memoryTrace
	}
	return udiag.BorrowProcessTraceTags()
}

func (g *GlobalParams) traceModuleCellAllocated(cell *GparamsRefCell) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := g.moduleMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "alloc ", fmt.Sprintf("%p", cell), " src/util/gparams.cpp:192 params_ref ", strconv.FormatUint(gparamsRefCellBytes, 10), "\n")
		})
}

func (g *GlobalParams) traceModuleCellDeallocated(cell *GparamsRefCell) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := g.moduleMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return g.memoryProcess == nil || !g.memoryProcess.Finalizing() }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "dealloc ", fmt.Sprintf("%p", cell), " src/util/gparams.cpp:117\n")
		})
}

func (g *GlobalParams) traceModuleDescriptorAllocated(d *ParamDescrs, sourceLine int) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := g.moduleMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "alloc ", fmt.Sprintf("%p", d), " src/shell/gparams_register_modules.cpp:", strconv.Itoa(sourceLine), " param_descrs ", strconv.FormatUint(d.OwnerBytes, 10), "\n")
		})
}

func (g *GlobalParams) traceModuleDescriptorDeallocated(d *ParamDescrs, sourceLine int) {
	if !GlobalParamsSourceDebug {
		return
	}
	tags := g.moduleMemoryTraceTags()
	if !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return g.memoryProcess == nil || !g.memoryProcess.Finalizing() }, func(w io.Writer) error {
			return gparamsMemoryParts(w, "dealloc ", fmt.Sprintf("%p", d), " src/util/gparams.cpp:", strconv.Itoa(sourceLine), "\n")
		})
}
