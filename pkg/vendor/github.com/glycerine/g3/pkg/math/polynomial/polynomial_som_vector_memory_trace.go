package polynomial

// Source: src/math/polynomial/polynomial.cpp:2159-2183;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (v *PolynomialSOMVector) traceSOMChildAllocated(child *PolynomialSOMBuffer) {
	tags := v.storage.monomials.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "alloc "+fmt.Sprintf("%p", child)+
				" src/math/polynomial/polynomial.cpp:2166 som_buffer "+
				strconv.FormatUint(uint64(reflect.TypeOf(PolynomialSOMBuffer{}).Size()), 10)+"\n")
			return nil
		})
}

func (v *PolynomialSOMVector) traceSOMChildDeallocated(child *PolynomialSOMBuffer) {
	tags := v.storage.monomials.monomialTraceTags()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool {
			if tags.MemoryFinalizing() {
				return false
			}
			runtime := v.storage.integer.RuntimeOwner()
			return runtime == nil || runtime.ProcessOwner() == nil || !runtime.ProcessOwner().Finalizing()
		}, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+fmt.Sprintf("%p", child)+" src/math/polynomial/polynomial.cpp:2182\n")
			return nil
		})
}
