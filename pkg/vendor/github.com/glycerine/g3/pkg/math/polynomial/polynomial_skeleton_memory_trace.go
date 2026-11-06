package polynomial

// Source: src/math/polynomial/polynomial.cpp:4226;
// src/util/scoped_ptr_vector.h:38,46; src/util/util.h:171.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *PolynomialSkeleton) traceGCDSkeletonAllocated() {
	tags := s.owner.monomials.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", s)+
				" src/math/polynomial/polynomial.cpp:4226 skeleton "+
				strconv.FormatUint(uint64(reflect.TypeOf(PolynomialSkeleton{}).Size()), 10)+"\n")
			return nil
		})
}

func (s *PolynomialSkeleton) traceGCDSkeletonDeallocated(line int) {
	tags := s.owner.monomials.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	file := "src/util/scoped_ptr_vector.h"
	if line == 171 {
		file = "src/util/util.h"
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			if tags.MemoryFinalizing() {
				return false
			}
			runtime := s.owner.integer.RuntimeOwner()
			return runtime == nil || runtime.ProcessOwner() == nil || !runtime.ProcessOwner().Finalizing()
		}, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", s)+" "+file+":"+strconv.Itoa(line)+"\n")
			return nil
		})
}
