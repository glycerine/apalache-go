package ast

// Source: src/ast/decl_collector.cpp:118-128; src/util/top_sort.h:78-80.

import (
	"fmt"
	"io"
	"strconv"
	"unsafe"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (o *sortDependencyOrder) dependencyTraceTags() *udiag.TraceTags {
	if o.trace != nil {
		return o.trace
	}
	return udiag.BorrowProcessTraceTags()
}

func (o *sortDependencyOrder) dependencySetIdentity(i int) string {
	// The concrete Go slice may move on growth. The lexical order owner and
	// insertion slot identify the source-owned set until its destructor.
	return fmt.Sprintf("%p/%d", o, i)
}

func (o *sortDependencyOrder) traceDependencySetAllocated(i int) {
	tags := o.dependencyTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", o.dependencySetIdentity(i), " src/ast/decl_collector.cpp:127 sort_set ",
				strconv.FormatUint(uint64(unsafe.Sizeof([]ASTHandle(nil))), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (o *sortDependencyOrder) traceDependencySetsDeallocated() {
	tags := o.dependencyTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	for i := range o.roots {
		_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
			func() bool { return !o.owner.astMemoryFinalizing(tags) }, func(w io.Writer) error {
				for _, part := range [...]string{
					"dealloc ", o.dependencySetIdentity(i), " src/util/top_sort.h:79\n",
				} {
					_, _ = io.WriteString(w, part)
				}
				return nil
			})
	}
}
